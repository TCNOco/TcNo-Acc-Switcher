package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func catalogJSON(t *testing.T, entries map[string]map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"Platforms": entries})
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	return raw
}

func existingExe(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("exe"), 0o644); err != nil {
		t.Fatalf("write fake exe: %v", err)
	}
	return path
}

func TestSeedDisabledPlatformsForFirstLaunch_disabledByDefaultStaysOff(t *testing.T) {
	setTestAppData(t)
	installed := existingExe(t, "tcnotestdetected.exe")
	optOutInstalled := existingExe(t, "tcnotestoptout.exe")

	raw := catalogJSON(t, map[string]map[string]any{
		"Detected":  {"ExeLocationDefault": installed},
		"OptOut":    {"ExeLocationDefault": optOutInstalled, "DisabledByDefault": true},
		"NotInFile": {"ExeLocationDefault": filepath.Join(t.TempDir(), "tcnotestmissing.exe")},
	})
	names := []string{"Detected", "NotInFile", "OptOut"}

	settings := defaultSettings()
	(&PlatformService{}).seedDisabledPlatformsForFirstLaunch(&settings, raw, names)

	if slices.Contains(settings.DisabledPlatforms, "Detected") {
		t.Fatalf("installed platform was disabled: %v", settings.DisabledPlatforms)
	}
	if !slices.Contains(settings.DisabledPlatforms, "OptOut") {
		t.Fatalf("DisabledByDefault platform was auto-enabled: %v", settings.DisabledPlatforms)
	}
	if !slices.Contains(settings.DisabledPlatforms, "NotInFile") {
		t.Fatalf("uninstalled platform was enabled: %v", settings.DisabledPlatforms)
	}
}

// An opt-out platform is not evidence that detection worked, so a machine with
// only those installed still falls back to Steam instead of an empty home screen.
func TestSeedDisabledPlatformsForFirstLaunch_optOutDoesNotSuppressSteamFallback(t *testing.T) {
	setTestAppData(t)
	optOutInstalled := existingExe(t, "tcnotestoptout.exe")

	raw := catalogJSON(t, map[string]map[string]any{
		"Steam":  {"ExeLocationDefault": filepath.Join(t.TempDir(), "tcnoteststeam.exe")},
		"OptOut": {"ExeLocationDefault": optOutInstalled, "DisabledByDefault": true},
	})
	names := []string{"OptOut", "Steam"}

	settings := defaultSettings()
	(&PlatformService{}).seedDisabledPlatformsForFirstLaunch(&settings, raw, names)

	if slices.Contains(settings.DisabledPlatforms, "Steam") {
		t.Fatalf("Steam fallback did not fire: %v", settings.DisabledPlatforms)
	}
	if !slices.Contains(settings.DisabledPlatforms, "OptOut") {
		t.Fatalf("DisabledByDefault platform survived the fallback: %v", settings.DisabledPlatforms)
	}
}

func TestDisabledByDefaultPlatforms_shippedCatalog(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "Platforms.json"))
	if err != nil {
		t.Fatalf("read Platforms.json: %v", err)
	}
	got := disabledByDefaultPlatforms(raw)
	for _, name := range []string{"OBS Studio", "GeForce Now", "PS Remote Play", "Discord", "Discord Canary", "Discord PTB"} {
		if _, ok := got[name]; !ok {
			t.Errorf("%s should be DisabledByDefault", name)
		}
	}
	if _, ok := got["Steam"]; ok {
		t.Error("Steam must not be DisabledByDefault")
	}
}
