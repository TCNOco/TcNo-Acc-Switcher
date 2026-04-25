package platform

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestAppSettingsJSON_RoundTripTrayFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
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
	p := filepath.Join(t.TempDir(), settingsFileName)
	raw := []byte(`{"version":1,"language":"en-US","futureUnknown":42}`)
	if err := atomicWriteBytes(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(p)
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
