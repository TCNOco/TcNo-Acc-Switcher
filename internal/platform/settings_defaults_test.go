package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// defaultOnFields are the settings that default to on for a file written before
// they existed. Each one is a switch a user can turn off, so each one has to
// come back off after a save.
var defaultOnFields = []struct {
	name string
	get  func(AppSettings) bool
	set  func(*AppSettings, bool)
}{
	{"statsEnabled", func(s AppSettings) bool { return s.StatsEnabled }, func(s *AppSettings, v bool) { s.StatsEnabled = v }},
	{"statsShare", func(s AppSettings) bool { return s.StatsShare }, func(s *AppSettings, v bool) { s.StatsShare = v }},
	{"prereleaseUpdates", func(s AppSettings) bool { return s.PrereleaseUpdates }, func(s *AppSettings, v bool) { s.PrereleaseUpdates = v }},
	{"crashReportAutoSubmit", func(s AppSettings) bool { return s.CrashReportAutoSubmit }, func(s *AppSettings, v bool) { s.CrashReportAutoSubmit = v }},
	{"discordRpc", func(s AppSettings) bool { return s.DiscordRpc }, func(s *AppSettings, v bool) { s.DiscordRpc = v }},
	{"animationsEnabled", func(s AppSettings) bool { return s.AnimationsEnabled }, func(s *AppSettings, v bool) { s.AnimationsEnabled = v }},
	{"controllerSupportEnabled", func(s AppSettings) bool { return s.ControllerSupportEnabled }, func(s *AppSettings, v bool) { s.ControllerSupportEnabled = v }},
}

// Turning one of these off used to survive only until the next settings write of
// any kind: the save path asked "was this key in the JSON" of a struct that had
// never come from JSON, so every field it did not name in a hand-written list
// was answered "no" and reset to its default. crashReportAutoSubmit was the one
// missing from that list, and it uploads crash dumps.
func TestSwitchingADefaultOnSettingOffSurvivesASave(t *testing.T) {
	for _, field := range defaultOnFields {
		t.Run(field.name, func(t *testing.T) {
			dir := testExeDirWithPortable(t)

			s := defaultSettings()
			field.set(&s, false)
			if err := SaveAppSettings(dir, s); err != nil {
				t.Fatal(err)
			}

			raw, err := os.ReadFile(filepath.Join(PortableUserDataDir(dir), settingsFileName))
			if err != nil {
				t.Fatal(err)
			}
			var onDisk map[string]json.RawMessage
			if err := json.Unmarshal(raw, &onDisk); err != nil {
				t.Fatal(err)
			}
			if string(onDisk[field.name]) != "false" {
				t.Fatalf("%s written as %s, want false", field.name, onDisk[field.name])
			}

			loaded, err := LoadAppSettings(dir)
			if err != nil {
				t.Fatal(err)
			}
			if field.get(loaded) {
				t.Fatalf("%s came back on after a save and load", field.name)
			}
		})
	}
}

// The save path no longer applies defaults, so it must not undo the shape rules
// either - those are invariants, not preferences.
func TestSavingStillNormalisesShape(t *testing.T) {
	dir := testExeDirWithPortable(t)

	s := defaultSettings()
	s.Version = 0
	s.Language = ""
	s.AppBgAlignment = "nonsense"
	// Sharing rich presence with it switched off is not a state that means
	// anything, so saving has to settle it.
	s.DiscordRpc = false
	s.DiscordRpcShare = true

	if err := SaveAppSettings(dir, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAppSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version == 0 {
		t.Error("Version left at 0")
	}
	if loaded.Language != "en-US" {
		t.Errorf("Language = %q, want en-US", loaded.Language)
	}
	if loaded.AppBgAlignment == "nonsense" {
		t.Error("AppBgAlignment kept a value outside its set")
	}
	if loaded.DiscordRpcShare {
		t.Error("DiscordRpcShare stayed on with DiscordRpc off")
	}
}

// A file that predates a setting still gets it turned on, which is the whole
// reason the load path looks at the raw JSON.
func TestASettingAbsentFromTheFileStillDefaultsOn(t *testing.T) {
	dir := testExeDirWithPortable(t)
	path := filepath.Join(PortableUserDataDir(dir), settingsFileName)
	if err := os.WriteFile(path, []byte(`{"version":1,"language":"en-US"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadAppSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range defaultOnFields {
		if field.name == "prereleaseUpdates" && !defaultPrereleaseUpdates {
			continue
		}
		if !field.get(loaded) {
			t.Errorf("%s came back off for a file that never mentioned it", field.name)
		}
	}
}
