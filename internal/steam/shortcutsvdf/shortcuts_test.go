package shortcutsvdf

import (
	"bytes"
	"testing"
)

func TestMarshalEmptyListIsTheCanonicalEmptyFile(t *testing.T) {
	// Not zero bytes: Steam cannot parse an empty file and answers by clearing
	// the user's whole shortcut list.
	if got := MarshalShortcuts(nil); !bytes.Equal(got, emptyFile) {
		t.Fatalf("MarshalShortcuts(nil) = % x, want % x", got, emptyFile)
	}
}

func TestParseEmptyFileYieldsNoShortcuts(t *testing.T) {
	list, err := ParseShortcuts(emptyFile)
	if err != nil {
		t.Fatalf("ParseShortcuts: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("%d shortcuts, want none", len(list))
	}
}

func TestMarshalRenumbersKeysContiguously(t *testing.T) {
	// Steam renumbers on load, so a file that already agrees survives a Steam
	// restart unchanged.
	inner := &Node{Entries: []Entry{
		{Key: "0", Kind: KindMap, Sub: sampleEntry()},
		{Key: "3", Kind: KindMap, Sub: sampleEntry()},
		{Key: "7", Kind: KindMap, Sub: sampleEntry()},
	}}
	list, err := ParseShortcuts(Encode(&Node{Entries: []Entry{{Key: "shortcuts", Kind: KindMap, Sub: inner}}}))
	if err != nil {
		t.Fatalf("ParseShortcuts: %v", err)
	}
	root, err := Decode(MarshalShortcuts(list))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	shortcuts, _ := root.Get("shortcuts")
	var keys []string
	for _, e := range shortcuts.Sub.Entries {
		keys = append(keys, e.Key)
	}
	want := []string{"0", "1", "2"}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %v, want %v", keys, want)
		}
	}
}

func TestShortcutsRoundTripPreservesEveryField(t *testing.T) {
	raw := MarshalShortcuts([]*Node{sampleEntry(), sampleEntry()})
	list, err := ParseShortcuts(raw)
	if err != nil {
		t.Fatalf("ParseShortcuts: %v", err)
	}
	if got := MarshalShortcuts(list); !bytes.Equal(got, raw) {
		t.Fatalf("round trip changed the bytes:\n got % x\nwant % x", got, raw)
	}
}

func TestParseRejectsAFileWithoutAShortcutsMap(t *testing.T) {
	other := Encode(&Node{Entries: []Entry{{Key: "other", Kind: KindMap, Sub: &Node{}}}})
	if _, err := ParseShortcuts(other); err == nil {
		t.Fatal("ParseShortcuts accepted a file with no shortcuts map, want an error")
	}
}
