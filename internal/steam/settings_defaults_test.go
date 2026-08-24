package steam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

// A settings file missing a key must keep that key's default. Decoding onto a
// zero value instead turned every absent bool off and then wrote it back, which
// is how a partial file silently disabled AutoStart, the VAC and limited badges,
// the account username, the last-login date and profile info collection.
func TestLoadSettingsKeepsDefaultsForAbsentKeys(t *testing.T) {
	exeDir := t.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))

	dir, err := paths.SettingsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	partial := `{"FolderPath":"/somewhere/Steam"}`
	if err := os.WriteFile(filepath.Join(dir, settingsFileName), []byte(partial), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	def := defaultSettings()

	checks := map[string]struct{ got, want bool }{
		"AutoStart":                {got.AutoStart, def.AutoStart},
		"Steam_ShowVAC":            {got.SteamShowVAC, def.SteamShowVAC},
		"Steam_ShowLimited":        {got.SteamShowLimited, def.SteamShowLimited},
		"Steam_ShowLastLogin":      {got.SteamShowLastLogin, def.SteamShowLastLogin},
		"Steam_ShowAccUsername":    {got.SteamShowAccUsername, def.SteamShowAccUsername},
		"CollectInfo":              {got.CollectInfo, def.CollectInfo},
		"Steam_ShowMiniProfile":    {got.SteamShowMiniProfile, def.SteamShowMiniProfile},
		"Steam_ShowSteamGuardLock": {got.SteamShowSteamGuardLock, def.SteamShowSteamGuardLock},
		"AlwaysSwapOnShortcut":     {got.AlwaysSwapOnShortcut, def.AlwaysSwapOnShortcut},
	}
	for key, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want the default %v", key, c.got, c.want)
		}
	}
	if got.TrayAccNumber != def.TrayAccNumber {
		t.Errorf("TrayAccNumber = %d, want the default %d", got.TrayAccNumber, def.TrayAccNumber)
	}
	if got.FolderPath == "" {
		t.Error("the one key the file did set was lost")
	}
}

// A key the file sets explicitly still wins, including when the value is the
// zero one - that is the whole reason it cannot just be merged over.
func TestLoadSettingsHonoursExplicitFalseAndZero(t *testing.T) {
	exeDir := t.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))

	dir, err := paths.SettingsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	chosen := map[string]any{
		"AutoStart":            false,
		"Steam_ShowVAC":        false,
		"TrayAccNumber":        0,
		"AlwaysSwapOnShortcut": false,
	}
	blob, err := json.Marshal(chosen)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, settingsFileName), blob, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.AutoStart {
		t.Error("AutoStart: an explicit false was overridden by the default")
	}
	if got.SteamShowVAC {
		t.Error("Steam_ShowVAC: an explicit false was overridden by the default")
	}
	if got.AlwaysSwapOnShortcut {
		t.Error("AlwaysSwapOnShortcut: an explicit false was overridden by the default")
	}
	if got.TrayAccNumber != 0 {
		t.Errorf("TrayAccNumber = %d, want the explicit 0 that disables the tray MRU", got.TrayAccNumber)
	}
}
