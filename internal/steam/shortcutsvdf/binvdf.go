// Package shortcutsvdf reads and writes Steam's userdata/<id32>/config/shortcuts.vdf,
// the binary KeyValues file holding a user's non-Steam games.
//
// It is a separate codec rather than a use of steamvdf because that package skips
// four header bytes shortcuts.vdf does not have, and collapses every scalar to a
// string - which loses the int32/string distinction a write has to put back.
//
// Steam resets a shortcuts.vdf it cannot parse to the empty form, taking every
// shortcut the user has with it. That is why unrecognised fields are carried
// through untouched rather than dropped, and why Decode returns errors instead
// of guessing.
package shortcutsvdf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// Kind is a binary KeyValues type byte.
type Kind byte

const (
	KindMap        Kind = 0x00
	KindString     Kind = 0x01
	KindInt32      Kind = 0x02
	KindFloat32    Kind = 0x03
	KindPointer    Kind = 0x04
	KindWideString Kind = 0x05
	KindColor      Kind = 0x06
	KindUint64     Kind = 0x07
	KindEnd        Kind = 0x08
	KindInt64      Kind = 0x0A
	KindEndAlt     Kind = 0x0B
)

// maxDepth stops a file that is all opening map bytes from recursing until the
// stack gives out. Steam nests three deep (root, shortcuts, entry, tags).
const maxDepth = 32

var errTruncated = errors.New("shortcutsvdf: truncated")

// Entry is one key/value pair. Which of Str, Int, Raw and Sub carries the value
// is decided by Kind; Raw holds the value bytes of a kind this package does not
// model, so an unknown field survives a decode/encode cycle byte for byte.
type Entry struct {
	Key  string
	Kind Kind
	Str  string
	Int  int32
	Raw  []byte
	Sub  *Node
}

// Node is an ordered map. Order is kept because Steam writes a fixed field order
// and a rewritten file has to come back out matching it.
type Node struct {
	Entries []Entry
}

// Decode parses a whole shortcuts.vdf and returns its root map.
func Decode(b []byte) (*Node, error) {
	r := &reader{buf: b}
	root, err := r.readMap(0)
	if err != nil {
		return nil, err
	}
	return root, nil
}

// Encode serialises a root map back to file bytes.
func Encode(n *Node) []byte {
	var out []byte
	return appendMap(out, n)
}

type reader struct {
	buf []byte
	pos int
}

func (r *reader) readMap(depth int) (*Node, error) {
	if depth > maxDepth {
		return nil, errors.New("shortcutsvdf: nesting too deep")
	}
	node := &Node{}
	for {
		kind, err := r.byte()
		if err != nil {
			return nil, err
		}
		if Kind(kind) == KindEnd || Kind(kind) == KindEndAlt {
			return node, nil
		}
		key, err := r.cstring()
		if err != nil {
			return nil, err
		}
		entry := Entry{Key: key, Kind: Kind(kind)}
		switch Kind(kind) {
		case KindMap:
			sub, err := r.readMap(depth + 1)
			if err != nil {
				return nil, err
			}
			entry.Sub = sub
		case KindString:
			val, err := r.cstring()
			if err != nil {
				return nil, err
			}
			entry.Str = val
		case KindInt32:
			raw, err := r.take(4)
			if err != nil {
				return nil, err
			}
			entry.Int = int32(binary.LittleEndian.Uint32(raw))
		case KindWideString:
			raw, err := r.wideString()
			if err != nil {
				return nil, err
			}
			entry.Raw = raw
		case KindFloat32, KindPointer, KindColor:
			raw, err := r.take(4)
			if err != nil {
				return nil, err
			}
			entry.Raw = append([]byte(nil), raw...)
		case KindUint64, KindInt64:
			raw, err := r.take(8)
			if err != nil {
				return nil, err
			}
			entry.Raw = append([]byte(nil), raw...)
		default:
			// Guessing a length here would misalign the rest of the file, and a
			// misaligned write is what makes Steam wipe the list.
			return nil, fmt.Errorf("shortcutsvdf: unknown type byte 0x%02x for key %q", kind, key)
		}
		node.Entries = append(node.Entries, entry)
	}
}

func (r *reader) byte() (byte, error) {
	if r.pos >= len(r.buf) {
		return 0, errTruncated
	}
	b := r.buf[r.pos]
	r.pos++
	return b, nil
}

func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || r.pos+n > len(r.buf) {
		return nil, errTruncated
	}
	out := r.buf[r.pos : r.pos+n]
	r.pos += n
	return out, nil
}

func (r *reader) cstring() (string, error) {
	start := r.pos
	for r.pos < len(r.buf) {
		if r.buf[r.pos] == 0 {
			s := string(r.buf[start:r.pos])
			r.pos++
			return s, nil
		}
		r.pos++
	}
	return "", errTruncated
}

// wideString returns the UTF-16 bytes including their two-byte terminator, so
// Encode can replay them without knowing the encoding.
func (r *reader) wideString() ([]byte, error) {
	start := r.pos
	for r.pos+1 < len(r.buf) {
		if r.buf[r.pos] == 0 && r.buf[r.pos+1] == 0 {
			r.pos += 2
			return append([]byte(nil), r.buf[start:r.pos]...), nil
		}
		r.pos += 2
	}
	return nil, errTruncated
}

func appendMap(out []byte, n *Node) []byte {
	if n != nil {
		for _, e := range n.Entries {
			out = appendEntry(out, e)
		}
	}
	return append(out, byte(KindEnd))
}

func appendEntry(out []byte, e Entry) []byte {
	out = append(out, byte(e.Kind))
	out = appendCString(out, e.Key)
	switch e.Kind {
	case KindMap:
		out = appendMap(out, e.Sub)
	case KindString:
		out = appendCString(out, e.Str)
	case KindInt32:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(e.Int))
		out = append(out, buf[:]...)
	default:
		out = append(out, e.Raw...)
	}
	return out
}

func appendCString(out []byte, s string) []byte {
	return append(append(out, s...), 0)
}

// Get finds an entry by key, case-insensitively: KeyValues is case-insensitive
// and Steam has written both "AppName" and "appname" over the years.
func (n *Node) Get(key string) (Entry, bool) {
	if n == nil {
		return Entry{}, false
	}
	for _, e := range n.Entries {
		if strings.EqualFold(e.Key, key) {
			return e, true
		}
	}
	return Entry{}, false
}

// GetString returns a string value, or "" when the key is absent or another kind.
func (n *Node) GetString(key string) string {
	if e, ok := n.Get(key); ok && e.Kind == KindString {
		return e.Str
	}
	return ""
}

// GetInt32 returns an int32 value, or 0 when the key is absent or another kind.
func (n *Node) GetInt32(key string) int32 {
	if e, ok := n.Get(key); ok && e.Kind == KindInt32 {
		return e.Int
	}
	return 0
}

// SetString replaces a value in place, keeping the key spelling already on disk,
// and appends when there is nothing to replace.
func (n *Node) SetString(key, val string) {
	n.set(Entry{Key: key, Kind: KindString, Str: val})
}

// SetInt32 replaces a value in place, or appends when the key is absent.
func (n *Node) SetInt32(key string, val int32) {
	n.set(Entry{Key: key, Kind: KindInt32, Int: val})
}

// SetMap replaces or appends a nested map.
func (n *Node) SetMap(key string, sub *Node) {
	n.set(Entry{Key: key, Kind: KindMap, Sub: sub})
}

func (n *Node) set(next Entry) {
	for i, e := range n.Entries {
		if strings.EqualFold(e.Key, next.Key) {
			next.Key = e.Key
			n.Entries[i] = next
			return
		}
	}
	n.Entries = append(n.Entries, next)
}
