package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestStatsOptOutRoundTrips(t *testing.T) {
	setTestAppData(t)
	exeDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ResetPathSingletonsForTest(exeDir)
	settings := defaultSettings()
	settings.StatsEnabled = false
	settings.StatsShare = false
	if err := SaveAppSettings(exeDir, settings); err != nil {
		t.Fatal(err)
	}

	path, ok := settingsFilePathForTest(exeDir)
	if !ok {
		t.Fatal("saved settings file was not found")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"statsEnabled": false`)) || !bytes.Contains(raw, []byte(`"statsShare": false`)) {
		t.Fatalf("explicit stats opt-outs were omitted: %s", raw)
	}

	ResetPathSingletonsForTest(exeDir)
	loaded, err := LoadAppSettings(exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StatsEnabled || loaded.StatsShare {
		t.Fatalf("stats opt-out did not survive reload: enabled=%v share=%v", loaded.StatsEnabled, loaded.StatsShare)
	}
}

func TestLegacyWindowSettingsMigratesPlatformsAndStats(t *testing.T) {
	setTestAppData(t)
	exeDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userDataDir, err := DefaultUserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	legacySettings := []byte(`{
  "DisabledPlatforms": [],
  "EnabledBasicPlatforms": ["e"],
  "CollectStats": false,
  "ShareAnonymousStats": false
}`)
	if err := os.WriteFile(filepath.Join(userDataDir, legacyWindowSettingsFileName), legacySettings, 0o644); err != nil {
		t.Fatal(err)
	}
	legacyCatalog := []byte(`{"Version":"2025-11-09_00","Platforms":{
  "Epic Games":{"Identifiers":["e","epic"]},
  "GOG Galaxy":{"Identifiers":["g","gog"]}
}}`)
	if err := os.WriteFile(filepath.Join(userDataDir, "Platforms.json"), legacyCatalog, 0o644); err != nil {
		t.Fatal(err)
	}
	previousEmbedded := append([]byte(nil), embeddedPlatformsJSON...)
	t.Cleanup(func() { SetEmbeddedPlatformsJSON(previousEmbedded) })
	SetEmbeddedPlatformsJSON([]byte(`{"Version":"4.0.2","Platforms":{"Steam":{"Identifiers":["s","steam","valve"]}}}`))

	ResetPathSingletonsForTest(exeDir)
	if err := InitDataPaths(exeDir); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadAppSettings(exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if settings.StatsEnabled || settings.StatsShare {
		t.Fatalf("legacy stats opt-out was lost: enabled=%v share=%v", settings.StatsEnabled, settings.StatsShare)
	}
	if slices.Contains(settings.DisabledPlatforms, "Steam") {
		t.Fatalf("legacy enabled Steam was disabled: %v", settings.DisabledPlatforms)
	}
	if slices.Contains(settings.DisabledPlatforms, "Epic Games") {
		t.Fatalf("legacy enabled basic platform was disabled: %v", settings.DisabledPlatforms)
	}
	if !slices.Contains(settings.DisabledPlatforms, "GOG Galaxy") {
		t.Fatalf("legacy inactive basic platform was enabled: %v", settings.DisabledPlatforms)
	}

	loadedCatalog, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := parsePlatformNames(loadedCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(names, "Steam") {
		t.Fatalf("Steam descriptor was not restored: %v", names)
	}
	if _, err := os.Stat(filepath.Join(userDataDir, legacyWindowSettingsFileName)); err != nil {
		t.Fatalf("legacy settings should remain available for rollback: %v", err)
	}
}

func TestExistingSettingsTakePrecedenceOverLegacyWindowSettings(t *testing.T) {
	setTestAppData(t)
	exeDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userDataDir, err := DefaultUserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDataDir, legacyWindowSettingsFileName), []byte(`{"DisabledPlatforms":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDataDir, settingsFileName), []byte(`{
  "version": 1,
  "language": "en-US",
  "disabledPlatforms": ["Steam"],
  "statsEnabled": false,
  "statsShare": false
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ResetPathSingletonsForTest(exeDir)
	settings, err := LoadAppSettings(exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(settings.DisabledPlatforms, "Steam") {
		t.Fatalf("legacy settings overwrote current settings: %v", settings.DisabledPlatforms)
	}
}

func TestInstallerStatsPreferenceIsConsumed(t *testing.T) {
	setTestAppData(t)
	exeDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userDataDir, err := DefaultUserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(userDataDir, "SendAnonymousStats.no")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ResetPathSingletonsForTest(exeDir)
	if err := SaveAppSettings(exeDir, defaultSettings()); err != nil {
		t.Fatal(err)
	}
	if err := InitDataPaths(exeDir); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadAppSettings(exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if settings.StatsEnabled || settings.StatsShare {
		t.Fatalf("installer send preference applied incorrectly: enabled=%v share=%v", settings.StatsEnabled, settings.StatsShare)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("installer marker was not consumed: %v", err)
	}
}

func settingsFilePathForTest(exeDir string) (string, bool) {
	for _, candidate := range append(settingsSearchDirsForTest(exeDir), exeDir) {
		path := filepath.Join(candidate, settingsFileName)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, true
		}
	}
	return "", false
}

func settingsSearchDirsForTest(exeDir string) []string {
	dirs := []string{PortableUserDataDir(exeDir)}
	if dir, err := DefaultUserDataDir(); err == nil {
		dirs = append(dirs, dir)
	}
	return dirs
}
