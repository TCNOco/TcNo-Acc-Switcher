// Package sqliteread is a read-only reader for the slice of the SQLite file
// format that SQLITE: platform descriptors need: one scalar lookup over one
// table. It parses a file the app does not control, so every read is bounded
// against the page size and the file length, and anything it cannot handle is
// an error rather than a guess.
package sqliteread

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
)

// ErrPendingWrites reports that a sidecar file may hold newer rows than the main
// database. Callers must treat the main file's contents as stale, not merely
// missing. ErrWAL and ErrJournal name which sidecar, and both match it.
var ErrPendingWrites = errors.New("sqlite database has pending writes")

var (
	ErrWAL     = fmt.Errorf("%w in a non-empty write-ahead log", ErrPendingWrites)
	ErrJournal = fmt.Errorf("%w in a hot rollback journal", ErrPendingWrites)
)

const headerSize = 100

var magic = []byte("SQLite format 3\x00")

type kind uint8

const (
	kindNull kind = iota
	kindInt
	kindReal
	kindText
	kindBlob
)

type value struct {
	kind kind
	i    int64
	r    float64
	b    []byte
}

type reader struct {
	f        *os.File
	size     int64
	pageSize int
	usable   int
	pages    int
}

func open(path string) (*reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	head := make([]byte, headerSize)
	if _, err := f.ReadAt(head, 0); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("read header: %w", err)
	}
	if !strings.HasPrefix(string(head), string(magic)) {
		_ = f.Close()
		return nil, errors.New("not a sqlite database")
	}
	pageSize := int(binary.BigEndian.Uint16(head[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if pageSize < 512 || pageSize > 65536 || pageSize&(pageSize-1) != 0 {
		_ = f.Close()
		return nil, fmt.Errorf("bad page size %d", pageSize)
	}
	usable := pageSize - int(head[20])
	if usable < 480 {
		_ = f.Close()
		return nil, fmt.Errorf("bad usable page size %d", usable)
	}
	if head[19] > 2 {
		_ = f.Close()
		return nil, fmt.Errorf("unsupported read format %d", head[19])
	}
	switch head[19] {
	case 1:
		if journalIsHot(path + "-journal") {
			_ = f.Close()
			return nil, ErrJournal
		}
	case 2:
		if walHasFrames(path+"-wal", pageSize) {
			_ = f.Close()
			return nil, ErrWAL
		}
	}
	pages := int(st.Size() / int64(pageSize))
	if pages < 1 {
		_ = f.Close()
		return nil, errors.New("truncated database")
	}
	return &reader{f: f, size: st.Size(), pageSize: pageSize, usable: usable, pages: pages}, nil
}

// walHasFrames reports whether a -wal sidecar holds frames the main database may
// not have yet. Frames left behind by a checkpointed log carry a salt that no
// longer matches the log header, and are not newer data. Anything unrecognisable
// counts as newer data: refusing to read beats returning a stale value.
func walHasFrames(path string, pageSize int) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	const walHeader, frameHeader = 32, 24
	h := make([]byte, walHeader+frameHeader)
	if _, err := f.ReadAt(h, 0); err != nil {
		return false
	}
	magic := binary.BigEndian.Uint32(h[0:4])
	if magic != 0x377f0682 && magic != 0x377f0683 {
		return true
	}
	if int(binary.BigEndian.Uint32(h[8:12])) != pageSize {
		return true
	}
	return string(h[16:24]) == string(h[walHeader+8:walHeader+16])
}

// journalIsHot reports whether a -journal sidecar holds a transaction still
// owed to the main database, whose pages therefore may be mid-write or carry
// values a rollback has yet to undo. Journal modes that keep the file after a
// commit truncate or zero it rather than delete it, so only a file long enough
// to hold a header, with the header magic intact, counts.
func journalIsHot(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	const journalHeader = 28
	h := make([]byte, journalHeader)
	if _, err := f.ReadAt(h, 0); err != nil {
		return false
	}
	return binary.BigEndian.Uint64(h[0:8]) == 0xd9d505f920a163d7
}

func (r *reader) close() { _ = r.f.Close() }

func (r *reader) page(n int) ([]byte, error) {
	if n < 1 || n > r.pages {
		return nil, fmt.Errorf("page %d out of range (1-%d)", n, r.pages)
	}
	p := make([]byte, r.pageSize)
	if _, err := r.f.ReadAt(p, int64(n-1)*int64(r.pageSize)); err != nil {
		return nil, fmt.Errorf("read page %d: %w", n, err)
	}
	return p, nil
}

// varint decodes a SQLite variable-length integer, returning its value and width.
func varint(b []byte) (int64, int, bool) {
	var v uint64
	for i := 0; i < 8 && i < len(b); i++ {
		v = v<<7 | uint64(b[i]&0x7f)
		if b[i]&0x80 == 0 {
			return int64(v), i + 1, true
		}
	}
	if len(b) < 9 {
		return 0, 0, false
	}
	return int64(v<<8 | uint64(b[8])), 9, true
}

// walk visits every row of the table b-tree rooted at root, stopping early when
// fn returns true.
func (r *reader) walk(root int, fn func(rowid int64, rec []byte) (bool, error)) error {
	visited := map[int]bool{}
	stack := []int{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[n] {
			return fmt.Errorf("page %d revisited", n)
		}
		visited[n] = true
		p, err := r.page(n)
		if err != nil {
			return err
		}
		off := 0
		if n == 1 {
			off = headerSize
		}
		if off+12 > r.usable {
			return fmt.Errorf("page %d header out of range", n)
		}
		hdr := 8
		interior := false
		switch p[off] {
		case 0x0d:
		case 0x05:
			hdr, interior = 12, true
		default:
			return fmt.Errorf("page %d is not a table b-tree page (type %d)", n, p[off])
		}
		cells := int(binary.BigEndian.Uint16(p[off+3 : off+5]))
		if off+hdr+cells*2 > r.usable {
			return fmt.Errorf("page %d cell array out of range", n)
		}
		if interior {
			stack = append(stack, int(binary.BigEndian.Uint32(p[off+8:off+12])))
		}
		for i := 0; i < cells; i++ {
			ptr := int(binary.BigEndian.Uint16(p[off+hdr+i*2 : off+hdr+i*2+2]))
			if ptr < off+hdr || ptr >= r.usable {
				return fmt.Errorf("page %d cell %d pointer %d out of range", n, i, ptr)
			}
			if interior {
				if ptr+4 > r.usable {
					return fmt.Errorf("page %d cell %d truncated", n, i)
				}
				stack = append(stack, int(binary.BigEndian.Uint32(p[ptr:ptr+4])))
				continue
			}
			rowid, rec, err := r.leafCell(p[:r.usable], ptr)
			if err != nil {
				return fmt.Errorf("page %d cell %d: %w", n, i, err)
			}
			stop, err := fn(rowid, rec)
			if err != nil || stop {
				return err
			}
		}
	}
	return nil
}

// leafCell reads one table-leaf cell, following its overflow chain if any.
func (r *reader) leafCell(p []byte, ptr int) (int64, []byte, error) {
	total, n, ok := varint(p[ptr:])
	if !ok || total < 0 || total > r.size {
		return 0, nil, errors.New("bad payload length")
	}
	ptr += n
	rowid, n, ok := varint(p[ptr:])
	if !ok {
		return 0, nil, errors.New("bad rowid")
	}
	ptr += n

	maxLocal := r.usable - 35
	local := int(total)
	if local > maxLocal {
		minLocal := (r.usable-12)*32/255 - 23
		if local = minLocal + (int(total)-minLocal)%(r.usable-4); local > maxLocal {
			local = minLocal
		}
	}
	if ptr+local > len(p) {
		return 0, nil, errors.New("payload out of range")
	}
	rec := make([]byte, 0, total)
	rec = append(rec, p[ptr:ptr+local]...)
	if int64(local) == total {
		return rowid, rec, nil
	}

	if ptr+local+4 > len(p) {
		return 0, nil, errors.New("overflow pointer out of range")
	}
	next := int(binary.BigEndian.Uint32(p[ptr+local : ptr+local+4]))
	seen := map[int]bool{}
	for int64(len(rec)) < total {
		if next == 0 || seen[next] {
			return 0, nil, errors.New("broken overflow chain")
		}
		seen[next] = true
		op, err := r.page(next)
		if err != nil {
			return 0, nil, err
		}
		next = int(binary.BigEndian.Uint32(op[0:4]))
		chunk := r.usable - 4
		if rem := int(total) - len(rec); chunk > rem {
			chunk = rem
		}
		rec = append(rec, op[4:4+chunk]...)
	}
	return rowid, rec, nil
}

// decodeRecord splits a record into its column values.
func decodeRecord(rec []byte) ([]value, error) {
	hdrLen, n, ok := varint(rec)
	if !ok || hdrLen < int64(n) || hdrLen > int64(len(rec)) {
		return nil, errors.New("bad record header")
	}
	types := make([]int64, 0, 8)
	for p := n; int64(p) < hdrLen; {
		t, w, ok := varint(rec[p:int(hdrLen)])
		if !ok || t < 0 {
			return nil, errors.New("bad serial type")
		}
		types = append(types, t)
		p += w
	}
	out := make([]value, 0, len(types))
	body := rec[hdrLen:]
	pos := 0
	for _, t := range types {
		if pos > len(body) {
			return nil, errors.New("truncated record body")
		}
		v, w, err := decodeValue(t, body[pos:])
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		pos += w
	}
	return out, nil
}

var intWidths = [...]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 6, 6: 8}

func decodeValue(t int64, b []byte) (value, int, error) {
	switch {
	case t == 0:
		return value{kind: kindNull}, 0, nil
	case t >= 1 && t <= 6:
		w := intWidths[t]
		if len(b) < w {
			return value{}, 0, errors.New("truncated integer")
		}
		var v int64
		if b[0]&0x80 != 0 {
			v = -1
		}
		for _, c := range b[:w] {
			v = v<<8 | int64(c)
		}
		return value{kind: kindInt, i: v}, w, nil
	case t == 7:
		if len(b) < 8 {
			return value{}, 0, errors.New("truncated float")
		}
		return value{kind: kindReal, r: math.Float64frombits(binary.BigEndian.Uint64(b[:8]))}, 8, nil
	case t == 8 || t == 9:
		return value{kind: kindInt, i: t - 8}, 0, nil
	case t >= 12:
		w := int((t - 12) / 2)
		if len(b) < w {
			return value{}, 0, errors.New("truncated blob or text")
		}
		k := kindBlob
		if t&1 == 1 {
			k = kindText
		}
		return value{kind: k, b: b[:w]}, w, nil
	default:
		return value{}, 0, fmt.Errorf("reserved serial type %d", t)
	}
}
