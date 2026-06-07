//go:build windows

package steam

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/winutil"
)

const steamTestRegBase = `HKCU\Software\Valve\Steam`

func setAndRecordSteamReg(t *testing.T, valueName, data string) {
	t.Helper()
	k := steamTestRegBase + ":" + valueName
	if err := winutil.RegistryWrite(k, data); err != nil {
		t.Fatalf("RegistryWrite %s: %v", valueName, err)
	}
	t.Cleanup(func() { _ = winutil.RegistryDelete(k) })
}

// ---------------------------------------------------------------------------
// writeLoginUsersAndRegistry — full VDF mutation + registry write
// ---------------------------------------------------------------------------

func TestWriteLoginUsersAndRegistry(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)

	// Set up a valid loginusers.vdf with two accounts
	loginPath := filepath.Join(configDir, "loginusers.vdf")
	initialVDF := `"users"
{
	"76561198000000100"
	{
		"AccountName"		"player1"
		"PersonaName"		"Player One"
		"Timestamp"		"1700000000"
		"WantsOfflineMode"		"0"
		"MostRecent"		"1"
		"RememberPassword"		"1"
	}
	"76561198000000200"
	{
		"AccountName"		"player2"
		"PersonaName"		"Player Two"
		"Timestamp"		"1690000000"
		"WantsOfflineMode"		"0"
		"MostRecent"		"0"
		"RememberPassword"		"0"
	}
}
`
	os.WriteFile(loginPath, []byte(initialVDF), 0o644)

	// Switch to account 2
	if err := writeLoginUsersAndRegistry(dir, "76561198000000200"); err != nil {
		t.Fatalf("writeLoginUsersAndRegistry: %v", err)
	}

	// Verify loginusers.vdf was overwritten
	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}

	var foundRecent, foundPassword string
	for _, u := range users {
		if u.SteamID64 == "76561198000000200" {
			foundRecent = u.MostRecent
			foundPassword = u.RememberPassword
		}
		if u.SteamID64 == "76561198000000100" {
			if u.MostRecent != "0" {
				t.Error("old account should have MostRecent=0")
			}
			if u.RememberPassword != "0" {
				t.Error("old account should have RememberPassword=0")
			}
		}
	}
	if foundRecent != "1" {
		t.Errorf("selected account MostRecent = %q, want 1", foundRecent)
	}
	if foundPassword != "1" {
		t.Errorf("selected account RememberPassword = %q, want 1", foundPassword)
	}

	// Verify .vdf_last backup was created
	backupPath := filepath.Join(configDir, "loginusers.vdf_last")
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf(".vdf_last backup not created: %v", err)
	}

	// Verify registry: AutoLoginUser = "player2"
	regAutoUser, _, err := winutil.RegistryRead(steamTestRegBase + ":AutoLoginUser")
	if err != nil {
		t.Errorf("RegistryRead AutoLoginUser: %v", err)
	} else if regAutoUser != "player2" {
		t.Errorf("AutoLoginUser = %q, want player2", regAutoUser)
	}

	// Verify registry: RememberPassword = 1
	regRemPass, _, err := winutil.RegistryRead(steamTestRegBase + ":RememberPassword")
	if err != nil {
		t.Errorf("RegistryRead RememberPassword: %v", err)
	} else if regRemPass != uint32(1) {
		t.Errorf("RememberPassword = %v, want 1", regRemPass)
	}

	// Clean up registry values
	_ = winutil.RegistryDelete(steamTestRegBase + ":AutoLoginUser")
	_ = winutil.RegistryDelete(steamTestRegBase + ":RememberPassword")
}

// ---------------------------------------------------------------------------
// writeLoginUsersAndRegistry — "Add New" mode (empty selectedID64)
// ---------------------------------------------------------------------------

func TestWriteLoginUsersAndRegistry_AddNew(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)

	loginPath := filepath.Join(configDir, "loginusers.vdf")
	initialVDF := `"users"
{
	"76561198000000100"
	{
		"AccountName"		"player1"
		"PersonaName"		"Player One"
		"Timestamp"		"1700000000"
		"MostRecent"		"1"
		"RememberPassword"		"1"
	}
}
`
	os.WriteFile(loginPath, []byte(initialVDF), 0o644)

	// Add New: empty selectedID64 → AutoLoginUser is written as "" which deletes the value on Windows
	if err := writeLoginUsersAndRegistry(dir, ""); err != nil {
		t.Fatalf("writeLoginUsersAndRegistry: %v", err)
	}

	users, _ := ParseLoginUsers(loginPath)
	for _, u := range users {
		if u.MostRecent != "0" {
			t.Errorf("all MostRecent should be 0, got %s=%s", u.SteamID64, u.MostRecent)
		}
		if u.RememberPassword != "0" {
			t.Errorf("all RememberPassword should be 0, got %s=%s", u.SteamID64, u.RememberPassword)
		}
	}

	// RegistryWrite("", ...) on Windows calls deleteRegistryValueIfPresent — the value is removed.
	// This is correct behavior for Add New mode.
	_, _, err := winutil.RegistryRead(steamTestRegBase + ":AutoLoginUser")
	if err == nil {
		t.Error("AutoLoginUser should not exist after AddNew (empty string write deletes)")
	}

	_ = winutil.RegistryDelete(steamTestRegBase + ":AutoLoginUser")
	_ = winutil.RegistryDelete(steamTestRegBase + ":RememberPassword")
}

// ---------------------------------------------------------------------------
// RemoveSteamAccountFromVDF
// ---------------------------------------------------------------------------

func TestRemoveSteamAccountFromVDF(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)

	loginPath := filepath.Join(configDir, "loginusers.vdf")
	initialVDF := `"users"
{
	"76561198000000100"
	{
		"AccountName"		"keep"
		"PersonaName"		"Keep"
		"Timestamp"		"1"
	}
	"76561198000000200"
	{
		"AccountName"		"remove"
		"PersonaName"		"Remove"
		"Timestamp"		"1"
	}
}
`
	os.WriteFile(loginPath, []byte(initialVDF), 0o644)

	if err := RemoveSteamAccountFromVDF(dir, "76561198000000200"); err != nil {
		t.Fatalf("RemoveSteamAccountFromVDF: %v", err)
	}

	users, _ := ParseLoginUsers(loginPath)
	if len(users) != 1 {
		t.Fatalf("expected 1 user after removal, got %d", len(users))
	}
	if users[0].SteamID64 != "76561198000000100" || users[0].AccountName != "keep" {
		t.Errorf("wrong user kept: %v", users[0])
	}

	// .vdf_last backup should exist
	backupPath := filepath.Join(configDir, "loginusers.vdf_last")
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf(".vdf_last backup not created: %v", err)
	}
}
