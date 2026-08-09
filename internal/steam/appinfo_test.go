package steam

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// appInfoBuilder writes the binary shapes parseAppInfo has to read. Building the
// file rather than checking in a fixture keeps the v28/v29 difference - which is the
// whole reason this parser needs a version guard - visible in the test.
type appInfoBuilder struct {
	v29     bool
	strings []string
	index   map[string]uint32
	apps    bytes.Buffer
}

func newAppInfoBuilder(v29 bool) *appInfoBuilder {
	return &appInfoBuilder{v29: v29, index: map[string]uint32{}}
}

func (b *appInfoBuilder) key(name string) []byte {
	if !b.v29 {
		return append([]byte(name), 0)
	}
	idx, ok := b.index[name]
	if !ok {
		idx = uint32(len(b.strings))
		b.index[name] = idx
		b.strings = append(b.strings, name)
	}
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, idx)
	return out
}

func (b *appInfoBuilder) nested(name string) []byte {
	return append([]byte{vdfNested}, b.key(name)...)
}

func (b *appInfoBuilder) str(name, value string) []byte {
	out := append([]byte{vdfString}, b.key(name)...)
	return append(out, append([]byte(value), 0)...)
}

func (b *appInfoBuilder) i32(name string, value int32) []byte {
	out := append([]byte{vdfInt32}, b.key(name)...)
	v := make([]byte, 4)
	binary.LittleEndian.PutUint32(v, uint32(value))
	return append(out, v...)
}

// addApp writes one record: the fixed header the format puts before every blob,
// then the blob itself.
func (b *appInfoBuilder) addApp(appID uint32, blob []byte) {
	headerLen := 40
	if b.v29 {
		headerLen += 20
	}
	var rec bytes.Buffer
	rec.Write(make([]byte, headerLen))
	rec.Write(blob)

	binary.Write(&b.apps, binary.LittleEndian, appID)
	binary.Write(&b.apps, binary.LittleEndian, uint32(rec.Len()))
	b.apps.Write(rec.Bytes())
}

func (b *appInfoBuilder) build() []byte {
	var out bytes.Buffer
	magic := uint32(appInfoMagicV28)
	if b.v29 {
		magic = appInfoMagicV29
	}
	binary.Write(&out, binary.LittleEndian, magic)
	binary.Write(&out, binary.LittleEndian, uint32(1)) // universe

	if !b.v29 {
		out.Write(b.apps.Bytes())
		binary.Write(&out, binary.LittleEndian, uint32(0)) // terminator
		return out.Bytes()
	}

	// The string table offset is absolute, so it can only be filled in once the
	// header and every app record are sized.
	const offsetField = 8
	body := b.apps.Bytes()
	tableOffset := int64(offsetField + 8 + len(body) + 4)
	binary.Write(&out, binary.LittleEndian, tableOffset)
	out.Write(body)
	binary.Write(&out, binary.LittleEndian, uint32(0)) // terminator

	binary.Write(&out, binary.LittleEndian, uint32(len(b.strings)))
	for _, s := range b.strings {
		out.WriteString(s)
		out.WriteByte(0)
	}
	return out.Bytes()
}

// realisticApp mirrors the nesting Steam actually writes: appinfo > common > name,
// with an associations subtree whose entries each carry their own "name".
func realisticApp(b *appInfoBuilder, name, publisher string) []byte {
	var blob bytes.Buffer
	blob.Write(b.nested("appinfo"))
	blob.Write(b.nested("common"))
	blob.Write(b.str("name", name))
	blob.Write(b.str("type", "game"))
	blob.Write(b.nested("associations"))
	blob.Write(b.nested("0"))
	blob.Write(b.str("type", "publisher"))
	blob.Write(b.str("name", publisher))
	blob.WriteByte(vdfEnd) // 0
	blob.WriteByte(vdfEnd) // associations
	blob.Write(b.i32("steam_release_date", 1234567890))
	blob.WriteByte(vdfEnd) // common
	blob.Write(b.nested("config"))
	blob.Write(b.str("installdir", "Whatever"))
	blob.WriteByte(vdfEnd) // config
	blob.WriteByte(vdfEnd) // appinfo
	return blob.Bytes()
}

func TestParseAppInfoReadsNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		v29  bool
	}{{"v28 inline keys", false}, {"v29 string table", true}} {
		t.Run(tc.name, func(t *testing.T) {
			b := newAppInfoBuilder(tc.v29)
			b.addApp(730, realisticApp(b, "Counter-Strike 2", "Valve"))
			b.addApp(228980, realisticApp(b, "Steamworks Common Redistributables", "Valve"))

			got, err := parseAppInfo(b.build())
			if err != nil {
				t.Fatal(err)
			}
			if got["730"] != "Counter-Strike 2" {
				t.Fatalf("app name mismatch: %q", got["730"])
			}
			if got["228980"] != "Steamworks Common Redistributables" {
				t.Fatalf("hidden app name mismatch: %q", got["228980"])
			}
		})
	}
}

// The bug this guards: a depth-blind search finds common/associations/0/name and
// reports the publisher as the game's name.
func TestParseAppInfoIgnoresAssociationNames(t *testing.T) {
	b := newAppInfoBuilder(true)
	b.addApp(730, realisticApp(b, "Counter-Strike 2", "Valve"))

	got, err := parseAppInfo(b.build())
	if err != nil {
		t.Fatal(err)
	}
	if got["730"] == "Valve" {
		t.Fatal("publisher was read as the app name")
	}
	if got["730"] != "Counter-Strike 2" {
		t.Fatalf("app name mismatch: %q", got["730"])
	}
}

func TestParseAppInfoRejectsUnknownVersion(t *testing.T) {
	raw := make([]byte, 32)
	binary.LittleEndian.PutUint32(raw, 0x07564430) // a version that does not exist yet
	if _, err := parseAppInfo(raw); err == nil {
		t.Fatal("expected an unsupported-version error")
	}
}

// One unreadable record must not cost the rest of the file. Records are skipped by
// their declared size, so a blob the VDF walk cannot follow only loses that app.
func TestParseAppInfoSkipsUnreadableRecord(t *testing.T) {
	b := newAppInfoBuilder(true)
	b.addApp(730, realisticApp(b, "Counter-Strike 2", "Valve"))
	b.addApp(999, []byte{0x7F, 0x7F, 0x7F, 0x7F, 0x7F, 0x7F, 0x7F, 0x7F}) // unknown tag
	b.addApp(570, realisticApp(b, "Dota 2", "Valve"))

	got, err := parseAppInfo(b.build())
	if err != nil {
		t.Fatal(err)
	}
	if got["730"] != "Counter-Strike 2" || got["570"] != "Dota 2" {
		t.Fatalf("a bad record cost its neighbours: %v", got)
	}
	if _, ok := got["999"]; ok {
		t.Fatal("the unreadable record should not have produced a name")
	}
}

// Steam rewrites this file underneath us. A v28 image cut short keeps whatever
// parsed; a v29 image cut short loses the string table its keys point into, so
// there is nothing to salvage and the caller falls back to the catalogue map.
func TestParseAppInfoTruncated(t *testing.T) {
	v28 := newAppInfoBuilder(false)
	v28.addApp(730, realisticApp(v28, "Counter-Strike 2", "Valve"))
	v28.addApp(570, realisticApp(v28, "Dota 2", "Valve"))
	full := v28.build()

	got, err := parseAppInfo(full[:len(full)-12])
	if err != nil {
		t.Fatal(err)
	}
	if got["730"] != "Counter-Strike 2" {
		t.Fatalf("v28 truncation should keep the records before the cut: %v", got)
	}

	v29 := newAppInfoBuilder(true)
	v29.addApp(730, realisticApp(v29, "Counter-Strike 2", "Valve"))
	cut := v29.build()
	if _, err := parseAppInfo(cut[:len(cut)-4]); err == nil {
		t.Fatal("v29 without its string table should be reported as unusable")
	}
}

func TestParseAppInfoEmptyAndGarbage(t *testing.T) {
	if _, err := parseAppInfo(nil); err == nil {
		t.Fatal("expected an error for an empty image")
	}
	if _, err := parseAppInfo([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected an error for a short image")
	}
}

// The appinfo cache is the last stop: it only answers for an app neither the
// manifest nor the catalogue map could name.
func TestResolveAppNameFallbackOrder(t *testing.T) {
	names := map[string]string{"730": "Counter-Strike 2"}
	local := map[string]string{"730": "Counter-Strike 2 (stale local)", "584210": "Ghost Recon Wildlands Open Beta"}

	if got := resolveAppName(names, local, "730"); got != "Counter-Strike 2" {
		t.Fatalf("catalogue map should win over the local cache: %q", got)
	}
	if got := resolveAppName(names, local, "584210"); got != "Ghost Recon Wildlands Open Beta" {
		t.Fatalf("local cache should answer what the map cannot: %q", got)
	}
	if got := resolveAppName(names, local, "999999"); got != "App 999999" {
		t.Fatalf("unknown app should fall back to the id: %q", got)
	}
	if got := resolveAppName(nil, nil, "12"); got != "App 12" {
		t.Fatalf("nil sources should not panic: %q", got)
	}
}
