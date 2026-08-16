package steam

import (
	"errors"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steam/accountstore"
)

// deadConfiguredFolder reproduces the shipped default on a machine with Steam
// somewhere else: SteamSettings.json names a folder that does not exist.
func deadConfiguredFolder(t *testing.T, env *steamTestEnv) {
	t.Helper()
	st, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	st.FolderPath = filepath.Join(env.exeDir, "Program Files (x86)", "Steam")
	if err := SaveSettings(st); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
}

// The configured folder outranks every source that would have found the real
// Steam, so an install on another drive listed no accounts at all.
func TestSteamAccountsListFindsSteamOutsideTheConfiguredFolder(t *testing.T) {
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)
	deadConfiguredFolder(t, env)

	app, err := platform.LoadAppSettings(env.exeDir)
	if err != nil {
		t.Fatalf("LoadAppSettings: %v", err)
	}
	if app.PlatformExePaths == nil {
		app.PlatformExePaths = map[string]string{}
	}
	app.PlatformExePaths["Steam"] = filepath.Join(env.steamDir, "steam.exe")
	if err := platform.SaveAppSettings(env.exeDir, app); err != nil {
		t.Fatalf("SaveAppSettings: %v", err)
	}

	rows, err := NewSteamService().GetSteamAccountsList()
	if err != nil {
		t.Fatalf("GetSteamAccountsList: %v", err)
	}
	if len(rows) != 1 || rows[0].SteamID64 != knownIDA {
		t.Fatalf("rows = %+v, want the account from the real Steam folder", rows)
	}
}

func TestSteamAccountsListReportsAMissingLoginFile(t *testing.T) {
	env := newSteamTestEnv(t)
	deadConfiguredFolder(t, env)

	rows, err := NewSteamService().GetSteamAccountsList()
	if !errors.Is(err, ErrSteamNotFound) {
		t.Fatalf("err = %v, want ErrSteamNotFound", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %+v, want none alongside the error", rows)
	}
}

// The store is the only copy of accounts Steam has already forgotten, so a
// login file nobody can find must not turn into an error over the top of them.
func TestSteamAccountsListKeepsStoredAccountsWithoutALoginFile(t *testing.T) {
	env := newSteamTestEnv(t)
	deadConfiguredFolder(t, env)

	if _, err := accountstore.Upsert(accountstore.Record{
		SteamID64:   knownIDB,
		AccountName: "vault_only",
		Source:      accountstore.SourceSteamGuard,
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := NewSteamService().GetSteamAccountsList()
	if err != nil {
		t.Fatalf("GetSteamAccountsList: %v", err)
	}
	if len(rows) != 1 || rows[0].SteamID64 != knownIDB {
		t.Fatalf("rows = %+v, want the stored account", rows)
	}
}
