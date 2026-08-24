package shortcutsvdf

import (
	"bytes"
	"testing"
)

// emptyFile is what Steam writes for a user with no non-Steam games: the root
// map holding an empty "shortcuts" map, and nothing else.
var emptyFile = []byte{
	0x00, 's', 'h', 'o', 'r', 't', 'c', 'u', 't', 's', 0x00,
	0x08,
	0x08,
}

func TestDecodeEmptyFile(t *testing.T) {
	root, err := Decode(emptyFile)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	e, ok := root.Get("shortcuts")
	if !ok || e.Kind != KindMap {
		t.Fatalf("root = %+v, want a shortcuts map", root.Entries)
	}
	if len(e.Sub.Entries) != 0 {
		t.Fatalf("shortcuts holds %d entries, want none", len(e.Sub.Entries))
	}
	if got := Encode(root); !bytes.Equal(got, emptyFile) {
		t.Fatalf("re-encoded % x, want % x", got, emptyFile)
	}
}

func TestEmptyStringIsNotAnEmptyMap(t *testing.T) {
	// The distinction steamvdf loses. Writing an empty icon back as a map would
	// change the shape of the file Steam reads.
	raw := []byte{0x00, 'r', 0x00, 0x01, 'i', 'c', 'o', 'n', 0x00, 0x00, 0x08, 0x08}
	root, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	sub, _ := root.Get("r")
	e, ok := sub.Sub.Get("icon")
	if !ok || e.Kind != KindString || e.Str != "" {
		t.Fatalf("icon = %+v, want an empty string", e)
	}
	if got := Encode(root); !bytes.Equal(got, raw) {
		t.Fatalf("re-encoded % x, want % x", got, raw)
	}
}

func TestInt32KeepsItsType(t *testing.T) {
	raw := []byte{0x02, 'I', 's', 'H', 'i', 'd', 'd', 'e', 'n', 0x00, 0x01, 0x00, 0x00, 0x00, 0x08}
	root, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := root.GetInt32("IsHidden"); got != 1 {
		t.Fatalf("IsHidden = %d, want 1", got)
	}
	if got := Encode(root); !bytes.Equal(got, raw) {
		t.Fatalf("re-encoded % x, want % x", got, raw)
	}
}

func TestNegativeAppIDSurvivesRoundTrip(t *testing.T) {
	// Every generated appid has its high bit set, so this is the normal case.
	root := &Node{}
	root.SetInt32("appid", -2119177653)
	decoded, err := Decode(Encode(root))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := decoded.GetInt32("appid"); got != -2119177653 {
		t.Fatalf("appid = %d, want -2119177653", got)
	}
}

func TestUnmodelledKindIsPreservedVerbatim(t *testing.T) {
	// A uint64 field, which this package has no accessor for. Steam has added
	// fields before; carrying them through unread is what stops us dropping one.
	raw := []byte{
		0x07, 'b', 'i', 'g', 0x00, 1, 2, 3, 4, 5, 6, 7, 8,
		0x08,
	}
	root, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := Encode(root); !bytes.Equal(got, raw) {
		t.Fatalf("re-encoded % x, want % x", got, raw)
	}
}

func TestAltEndMarkerCloses(t *testing.T) {
	raw := []byte{0x01, 'k', 0x00, 'v', 0x00, 0x0B}
	root, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := root.GetString("k"); got != "v" {
		t.Fatalf("k = %q, want %q", got, "v")
	}
}

func TestTruncatedInputErrorsWithoutPanicking(t *testing.T) {
	full := MarshalShortcuts([]*Node{sampleEntry()})
	for i := 0; i < len(full); i++ {
		if _, err := Decode(full[:i]); err == nil {
			t.Fatalf("Decode of %d-byte prefix succeeded, want an error", i)
		}
	}
}

func TestUnknownTypeByteIsAnError(t *testing.T) {
	// Guessing a length would misalign the rest of the file, and Steam clears the
	// list when it cannot parse one.
	if _, err := Decode([]byte{0x7f, 'k', 0x00, 0x08}); err == nil {
		t.Fatal("Decode accepted type byte 0x7f, want an error")
	}
}

func TestSetReplacesInPlaceKeepingDiskSpelling(t *testing.T) {
	root := &Node{}
	root.SetString("appname", "old")
	root.SetString("AppName", "new")
	if len(root.Entries) != 1 {
		t.Fatalf("%d entries, want 1", len(root.Entries))
	}
	if root.Entries[0].Key != "appname" {
		t.Fatalf("key = %q, want the spelling already on disk", root.Entries[0].Key)
	}
	if root.Entries[0].Str != "new" {
		t.Fatalf("value = %q, want %q", root.Entries[0].Str, "new")
	}
}

func sampleEntry() *Node {
	n := &Node{}
	n.SetInt32("appid", -2119177653)
	n.SetString("AppName", "TcNo Account Switcher")
	n.SetString("Exe", `"C:\Apps\TcNo-Acc-Switcher.exe"`)
	n.SetString("StartDir", `"C:\Apps\"`)
	n.SetString("icon", "")
	n.SetInt32("IsHidden", 0)
	n.SetMap("tags", &Node{})
	return n
}
