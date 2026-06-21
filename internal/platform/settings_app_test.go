package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testExeDirWithPortable(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(PortableUserDataDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	ResetPathSingletonsForTest(dir)
	return dir
}

func TestAppSettingsJSON_RoundTripTrayFields(t *testing.T) {
	t.Parallel()
	dir := testExeDirWithPortable(t)
	s := AppSettings{
		Version:          1,
		Language:         "en-US",
		ExitToTray:       true,
		MinimizeOnSwitch: true,
		PlatformExePaths: map[string]string{},
	}
	if err := SaveAppSettings(dir, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAppSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ExitToTray || !loaded.MinimizeOnSwitch {
		t.Fatalf("got %+v", loaded)
	}
}

func TestAppSettingsJSON_UnknownFieldsIgnored(t *testing.T) {
	t.Parallel()
	dir := testExeDirWithPortable(t)
	p := filepath.Join(PortableUserDataDir(dir), settingsFileName)
	raw := []byte(`{"version":1,"language":"en-US","futureUnknown":42}`)
	if err := atomicWriteBytes(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadAppSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.ExitToTray || s.MinimizeOnSwitch {
		t.Fatalf("defaults should be false, got %+v", s)
	}
}

func TestAppSettings_UnmarshalEmptyObject(t *testing.T) {
	t.Parallel()
	var s AppSettings
	if err := json.Unmarshal([]byte(`{}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.ExitToTray || s.MinimizeOnSwitch {
		t.Fatalf("expected false defaults, got %+v", s)
	}
}

func TestAppSettingsJSON_AnimationsEnabled(t *testing.T) {
	t.Parallel()

	// Round-trip: save false, reload, expect false
	dir := testExeDirWithPortable(t)
	s := AppSettings{
		Version:           1,
		Language:          "en-US",
		AnimationsEnabled: false,
		PlatformExePaths:  map[string]string{},
	}
	if err := SaveAppSettings(dir, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAppSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AnimationsEnabled {
		t.Fatalf("expected AnimationsEnabled=false after round-trip, got %+v", loaded)
	}

	// Missing key should default to true
	dir2 := testExeDirWithPortable(t)
	p := filepath.Join(PortableUserDataDir(dir2), settingsFileName)
	raw := []byte(`{"version":1,"language":"en-US"}`)
	if err := atomicWriteBytes(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded2, err := LoadAppSettings(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded2.AnimationsEnabled {
		t.Fatalf("expected AnimationsEnabled=true when omitted, got %+v", loaded2)
	}
}
