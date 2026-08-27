package steam

import (
	"encoding/binary"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// appcache/appinfo.vdf is the Steam client's own metadata cache. It carries a name
// for every app the client has fetched info for, which is a superset of what is
// installed and includes the classes no public endpoint will name at all: betas,
// staging branches, drivers, redistributables and hard-delisted apps.
//
// It is the last stop before "App <id>". The catalogue map is preferred over it
// because the map is refreshed against the store on its own schedule while this file
// only holds what this machine happened to look at.
//
// The format is undocumented and Valve has revised it. Every failure here is soft:
// an unknown version, a short read or a malformed record yields no names rather than
// an error, and the caller falls back exactly as it would with no Steam install at all.
const (
	appInfoMagicV28 = 0x07564428
	appInfoMagicV29 = 0x07564429

	// The file is ~17MB on a well-used install. The cap is here because this is a
	// file Steam owns, not us.
	appInfoMaxBytes = 128 << 20

	// Guards against a malformed size field walking the reader off into the weeds.
	appInfoMaxApps = 1 << 20
)

var errAppInfoUnsupported = errors.New("unsupported appinfo.vdf version")

// appInfoCache memoises the parse against the file's identity. Parsing 17MB measures
// at tens of milliseconds, which is cheap but not free on a path that paints a screen.
var appInfoCache struct {
	sync.Mutex
	path    string
	modTime time.Time
	size    int64
	names   map[string]string
}

// appInfoNamesFn is a var for the same reason the other reach-past-the-process hooks
// in this package are: it reads whatever Steam is installed on the machine.
var appInfoNamesFn = loadAppInfoNames

// loadAppInfoNames returns app id -> name from the local Steam client cache, or an
// empty map when there is nothing usable to read.
func loadAppInfoNames() map[string]string {
	root, err := installRoot()
	if err != nil || root == "" {
		return map[string]string{}
	}
	path := filepath.Join(filepath.Clean(root), "appcache", "appinfo.vdf")

	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > appInfoMaxBytes {
		return map[string]string{}
	}

	appInfoCache.Lock()
	defer appInfoCache.Unlock()
	if appInfoCache.names != nil && appInfoCache.path == path &&
		appInfoCache.modTime.Equal(info.ModTime()) && appInfoCache.size == info.Size() {
		return appInfoCache.names
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		steamLog().Debug("steam appinfo cache unreadable", slog.Any("err", err))
		return map[string]string{}
	}
	names, err := parseAppInfo(raw)
	if err != nil {
		steamLog().Debug("steam appinfo cache not parsed", slog.Any("err", err), slog.Int("bytes", len(raw)))
		names = map[string]string{}
	} else {
		steamLog().Debug("steam appinfo cache parsed", slog.Int("names", len(names)), slog.Int("bytes", len(raw)))
	}

	appInfoCache.path = path
	appInfoCache.modTime = info.ModTime()
	appInfoCache.size = info.Size()
	appInfoCache.names = names
	return names
}

// parseAppInfo reads app id -> common/name out of an appinfo.vdf image.
//
// Records that do not parse are skipped rather than failing the whole file: a partial
// answer is worth more here than none, and the file is a cache that Steam rewrites
// underneath us.
func parseAppInfo(raw []byte) (map[string]string, error) {
	r := &byteReader{buf: raw}
	magic, ok := r.uint32()
	if !ok {
		return nil, errAppInfoUnsupported
	}
	if magic != appInfoMagicV28 && magic != appInfoMagicV29 {
		return nil, errAppInfoUnsupported
	}
	v29 := magic == appInfoMagicV29
	if _, ok := r.uint32(); !ok { // universe
		return nil, errAppInfoUnsupported
	}

	// v29 replaces inline key names with indices into a table at the end of the file.
	var stringTable []string
	if v29 {
		offset, ok := r.int64()
		if !ok || offset < 0 || offset > int64(len(raw)) {
			return nil, errAppInfoUnsupported
		}
		stringTable, ok = parseAppInfoStringTable(raw[offset:])
		if !ok {
			return nil, errAppInfoUnsupported
		}
	}

	// infoState, lastUpdated, picsToken, sha1, changeNumber, and on v29 a second sha1.
	headerLen := 40
	if v29 {
		headerLen += 20
	}

	names := make(map[string]string)
	for apps := 0; apps < appInfoMaxApps; apps++ {
		appID, ok := r.uint32()
		if !ok || appID == 0 {
			break
		}
		size, ok := r.uint32()
		if !ok || int(size) < headerLen || !r.has(int(size)) {
			break
		}
		record := r.take(int(size))
		if name, ok := appInfoRecordName(record[headerLen:], stringTable, v29); ok {
			names[strconv.FormatUint(uint64(appID), 10)] = name
		}
	}
	return names, nil
}

func parseAppInfoStringTable(raw []byte) ([]string, bool) {
	r := &byteReader{buf: raw}
	count, ok := r.uint32()
	if !ok || count > appInfoMaxApps {
		return nil, false
	}
	table := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		s, ok := r.cstring()
		if !ok {
			return nil, false
		}
		table = append(table, s)
	}
	return table, true
}

// Binary VDF tags.
const (
	vdfNested = 0x00
	vdfString = 0x01
	vdfInt32  = 0x02
	vdfFloat  = 0x03
	vdfPtr    = 0x04
	vdfWide   = 0x05
	vdfColor  = 0x06
	vdfUint64 = 0x07
	vdfEnd    = 0x08
	vdfInt64  = 0x0A
)

// appInfoRecordName pulls common/name out of one app's binary VDF blob.
//
// Only a direct child of common counts. common/associations holds publisher and
// developer entries that each carry their own "name", so a depth-blind search
// returns "Valve" where it should return "Counter-Strike 2".
func appInfoRecordName(blob []byte, stringTable []string, v29 bool) (string, bool) {
	r := &byteReader{buf: blob}
	depth := 0
	commonDepth := -1
	for {
		tag, ok := r.byte()
		if !ok {
			return "", false
		}
		if tag == vdfEnd {
			depth--
			if commonDepth >= 0 && depth < commonDepth {
				commonDepth = -1
			}
			if depth <= 0 {
				return "", false
			}
			continue
		}

		var key string
		if v29 {
			index, ok := r.uint32()
			if !ok || int(index) >= len(stringTable) {
				return "", false
			}
			key = stringTable[index]
		} else if key, ok = r.cstring(); !ok {
			return "", false
		}

		switch tag {
		case vdfNested:
			depth++
			if key == "common" && commonDepth < 0 {
				commonDepth = depth
			}
		case vdfString:
			value, ok := r.cstring()
			if !ok {
				return "", false
			}
			if commonDepth >= 0 && depth == commonDepth && key == "name" && value != "" {
				return value, true
			}
		case vdfWide:
			if !r.skipWide() {
				return "", false
			}
		case vdfInt32, vdfFloat, vdfPtr, vdfColor:
			if !r.skip(4) {
				return "", false
			}
		case vdfUint64, vdfInt64:
			if !r.skip(8) {
				return "", false
			}
		default:
			// An unknown tag means the walk has lost sync; the rest of this record
			// cannot be trusted, but other records still can.
			return "", false
		}
	}
}

type byteReader struct {
	buf []byte
	pos int
}

func (r *byteReader) has(n int) bool { return n >= 0 && len(r.buf)-r.pos >= n }

func (r *byteReader) take(n int) []byte {
	out := r.buf[r.pos : r.pos+n]
	r.pos += n
	return out
}

func (r *byteReader) skip(n int) bool {
	if !r.has(n) {
		return false
	}
	r.pos += n
	return true
}

func (r *byteReader) byte() (byte, bool) {
	if !r.has(1) {
		return 0, false
	}
	b := r.buf[r.pos]
	r.pos++
	return b, true
}

func (r *byteReader) uint32() (uint32, bool) {
	if !r.has(4) {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v, true
}

func (r *byteReader) int64() (int64, bool) {
	if !r.has(8) {
		return 0, false
	}
	v := int64(binary.LittleEndian.Uint64(r.buf[r.pos:]))
	r.pos += 8
	return v, true
}

func (r *byteReader) cstring() (string, bool) {
	for i := r.pos; i < len(r.buf); i++ {
		if r.buf[i] == 0 {
			s := string(r.buf[r.pos:i])
			r.pos = i + 1
			return s, true
		}
	}
	return "", false
}

func (r *byteReader) skipWide() bool {
	for i := r.pos; i+1 < len(r.buf); i += 2 {
		if r.buf[i] == 0 && r.buf[i+1] == 0 {
			r.pos = i + 2
			return true
		}
	}
	return false
}
