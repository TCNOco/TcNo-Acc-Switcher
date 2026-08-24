package shortcutsvdf

import (
	"errors"
	"strconv"
)

// rootKey is the single key of the root map, spelled lowercase by Steam.
const rootKey = "shortcuts"

// ParseShortcuts returns each shortcut in file order. A file holding no
// shortcuts yields no entries and no error.
func ParseShortcuts(b []byte) ([]*Node, error) {
	// Steam writes an empty file rather than deleting it; treat both as "none".
	if len(b) == 0 {
		return nil, nil
	}
	root, err := Decode(b)
	if err != nil {
		return nil, err
	}
	entry, ok := root.Get(rootKey)
	if !ok || entry.Kind != KindMap {
		return nil, errors.New("shortcutsvdf: no shortcuts map")
	}
	var out []*Node
	for _, e := range entry.Sub.Entries {
		if e.Kind != KindMap {
			continue
		}
		out = append(out, e.Sub)
	}
	return out, nil
}

// MarshalShortcuts renumbers the entries to contiguous "0", "1", ... - which is
// what Steam does on load anyway - and wraps them in the root map.
//
// Marshalling an empty list yields the canonical 13-byte empty file. Writing
// zero bytes instead would be a file Steam cannot parse, and it answers that by
// clearing the list.
func MarshalShortcuts(list []*Node) []byte {
	inner := &Node{}
	for i, sc := range list {
		if sc == nil {
			continue
		}
		inner.Entries = append(inner.Entries, Entry{
			Key:  strconv.Itoa(i),
			Kind: KindMap,
			Sub:  sc,
		})
	}
	return Encode(&Node{Entries: []Entry{{Key: rootKey, Kind: KindMap, Sub: inner}}})
}
