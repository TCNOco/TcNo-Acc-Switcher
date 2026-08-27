package steam

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

// Compact app array, v1: "TCAA", version byte, uvarint record count, then one
// record per app - uvarint id delta from the previous id, uvarint name length,
// the UTF-8 name. Ids ascend, so every delta after the first is >= 1.
const (
	compactAppArrayMagic   = "TCAA"
	compactAppArrayVersion = 1

	// Limits for a payload that arrives over the network into an admin-installed
	// process. The live array is ~259k records and ~5.5 MB decoded.
	compactAppArrayMaxBytes    = 64 << 20
	compactAppArrayMaxCount    = 5_000_000
	compactAppArrayMaxNameLen  = 4096
	compactAppArrayMaxPrealloc = 1 << 20
)

func parseAppNameMapCompact(raw []byte) (map[string]string, error) {
	m, err := decodeCompactAppArray(raw)
	if err != nil {
		return nil, err
	}
	if !steamAppNameMapLooksValid(m) {
		return nil, fmt.Errorf("steam app name map invalid")
	}
	return m, nil
}

// decodeCompactAppArray returns the same map[decimal app id]name shape as
// parseAppNameMapJSON, so every consumer downstream is unchanged.
func decodeCompactAppArray(raw []byte) (map[string]string, error) {
	if len(raw) > compactAppArrayMaxBytes {
		return nil, fmt.Errorf("compact app array: %d bytes over %d limit", len(raw), compactAppArrayMaxBytes)
	}
	if len(raw) < len(compactAppArrayMagic)+1 {
		return nil, fmt.Errorf("compact app array: truncated header (%d bytes)", len(raw))
	}
	if got := string(raw[:len(compactAppArrayMagic)]); got != compactAppArrayMagic {
		return nil, fmt.Errorf("compact app array: bad magic %q", got)
	}
	if v := raw[len(compactAppArrayMagic)]; v != compactAppArrayVersion {
		return nil, fmt.Errorf("compact app array: unsupported version %d", v)
	}
	rest := raw[len(compactAppArrayMagic)+1:]

	count, n := binary.Uvarint(rest)
	if n <= 0 {
		return nil, fmt.Errorf("compact app array: bad record count")
	}
	rest = rest[n:]
	if count > compactAppArrayMaxCount {
		return nil, fmt.Errorf("compact app array: record count %d over %d limit", count, compactAppArrayMaxCount)
	}

	m := make(map[string]string, compactAppArrayPrealloc(count, len(rest)))
	var id uint64
	for i := uint64(0); i < count; i++ {
		delta, n := binary.Uvarint(rest)
		if n <= 0 {
			return nil, fmt.Errorf("compact app array: bad id delta at record %d", i)
		}
		rest = rest[n:]
		if i > 0 && delta == 0 {
			return nil, fmt.Errorf("compact app array: non-ascending id at record %d", i)
		}
		if id+delta < id {
			return nil, fmt.Errorf("compact app array: id overflow at record %d", i)
		}
		id += delta

		nameLen, n := binary.Uvarint(rest)
		if n <= 0 {
			return nil, fmt.Errorf("compact app array: bad name length at record %d", i)
		}
		rest = rest[n:]
		if nameLen > compactAppArrayMaxNameLen {
			return nil, fmt.Errorf("compact app array: name length %d at record %d over %d limit", nameLen, i, compactAppArrayMaxNameLen)
		}
		if uint64(len(rest)) < nameLen {
			return nil, fmt.Errorf("compact app array: name at record %d runs past end of buffer", i)
		}
		m[strconv.FormatUint(id, 10)] = string(rest[:nameLen])
		rest = rest[nameLen:]
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("compact app array: %d trailing bytes", len(rest))
	}
	return m, nil
}

// compactAppArrayPrealloc sizes the map from the count header without trusting
// it: a record is at least two bytes, so the body itself bounds how many can
// really follow, and a hostile count then cannot reserve more than it carries.
func compactAppArrayPrealloc(count uint64, remaining int) int {
	if max := uint64(remaining / 2); count > max {
		count = max
	}
	if count > compactAppArrayMaxPrealloc {
		count = compactAppArrayMaxPrealloc
	}
	return int(count)
}
