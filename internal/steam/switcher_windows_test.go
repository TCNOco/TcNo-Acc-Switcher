//go:build windows

package steam

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steam/accountstore"
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
		if u.AutoLogin != "0" {
			t.Errorf("all AutoLogin should be 0, got %s=%s", u.SteamID64, u.AutoLogin)
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
// writeLoginUsersAndRegistry — accounts Steam does not know about
// ---------------------------------------------------------------------------

func seedStoredAccount(t *testing.T, rec accountstore.Record) {
	t.Helper()
	paths.ResetForTest(t.TempDir())
	if _, err := accountstore.Upsert(rec); err != nil {
		t.Fatalf("seed account store: %v", err)
	}
	t.Cleanup(func() {
		_ = winutil.RegistryDelete(steamTestRegBase + ":AutoLoginUser")
		_ = winutil.RegistryDelete(steamTestRegBase + ":RememberPassword")
	})
}

// A Steam Guard login-only account has never been in loginusers.vdf. Switching
// to it must add the row alongside the accounts Steam already knows.
func TestWriteLoginUsersAndRegistry_AppendsStoredAccount(t *testing.T) {
	seedStoredAccount(t, accountstore.Record{
		SteamID64:   "76561198000000200",
		AccountName: "vault_only",
		PersonaName: "Vault Only",
		Timestamp:   "1690000000",
		Source:      accountstore.SourceSteamGuard,
	})

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	loginPath := filepath.Join(configDir, "loginusers.vdf")
	os.WriteFile(loginPath, []byte(`"users"
{
	"76561198000000100"
	{
		"AccountName"		"player1"
		"PersonaName"		"Player One"
		"Timestamp"		"1700000000"
		"AutoLogin"		"1"
		"RememberPassword"		"1"
	}
}
`), 0o644)

	if err := writeLoginUsersAndRegistry(dir, "76561198000000200"); err != nil {
		t.Fatalf("writeLoginUsersAndRegistry: %v", err)
	}

	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d rows, want the existing account plus the re-created one", len(users))
	}
	if active := ActiveSessionSteamID64(users); active != "76561198000000200" {
		t.Errorf("ActiveSessionSteamID64 = %q, want the re-created account", active)
	}
	for _, u := range users {
		switch u.SteamID64 {
		case "76561198000000200":
			if u.AccountName != "vault_only" || u.PersonaName != "Vault Only" {
				t.Errorf("re-created row = %+v, want the stored names", u)
			}
			if u.Timestamp != "1690000000" {
				t.Errorf("re-created Timestamp = %q, want the stored one", u.Timestamp)
			}
		case "76561198000000100":
			if u.AccountName != "player1" || u.PersonaName != "Player One" {
				t.Errorf("existing row was damaged: %+v", u)
			}
			if u.AutoLogin != "0" || u.RememberPassword != "0" {
				t.Errorf("existing row still claims the session: %+v", u)
			}
		}
	}

	regAutoUser, _, err := winutil.RegistryRead(steamTestRegBase + ":AutoLoginUser")
	if err != nil {
		t.Fatalf("RegistryRead AutoLoginUser: %v", err)
	}
	if regAutoUser != "vault_only" {
		t.Errorf("AutoLoginUser = %q, want vault_only", regAutoUser)
	}
}

// Advanced Clearing deletes loginusers.vdf outright. A switch must rebuild it.
func TestWriteLoginUsersAndRegistry_RebuildsDeletedFile(t *testing.T) {
	seedStoredAccount(t, accountstore.Record{
		SteamID64:   "76561198000000100",
		AccountName: "player1",
		PersonaName: "Player One",
		Source:      accountstore.SourceVDF,
	})

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "config"), 0o755)

	if err := writeLoginUsersAndRegistry(dir, "76561198000000100"); err != nil {
		t.Fatalf("writeLoginUsersAndRegistry: %v", err)
	}

	users, err := ParseLoginUsers(filepath.Join(dir, "config", "loginusers.vdf"))
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	if len(users) != 1 || users[0].AccountName != "player1" {
		t.Fatalf("got %+v, want the single re-created account", users)
	}
	if users[0].AutoLogin != "1" || users[0].RememberPassword != "1" {
		t.Errorf("re-created row is not set up for login: %+v", users[0])
	}
}

// AutoLoginUser is the login name, so a stored account without one cannot be
// preselected. That has to degrade to Steam's account chooser with the row
// present, not fail the switch.
func TestWriteLoginUsersAndRegistry_NoAccountNameFallsBackToChooser(t *testing.T) {
	seedStoredAccount(t, accountstore.Record{
		SteamID64: "76561198000000300",
		Source:    accountstore.SourceSteamGuard,
	})

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	loginPath := filepath.Join(configDir, "loginusers.vdf")
	os.WriteFile(loginPath, []byte(`"users"
{
	"76561198000000100"
	{
		"AccountName"		"player1"
		"PersonaName"		"Player One"
	}
}
`), 0o644)

	if err := writeLoginUsersAndRegistry(dir, "76561198000000300"); err != nil {
		t.Fatalf("a nameless stored account should still switch: %v", err)
	}

	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	// No junk row: a nameless entry could never be auto-logged-in and would be
	// dropped on the next read, so Steam's file keeps only the real account.
	if len(users) != 1 || users[0].SteamID64 != "76561198000000100" {
		t.Fatalf("got %+v, want the named account untouched", users)
	}
	if users[0].AutoLogin != "0" || users[0].RememberPassword != "0" {
		t.Errorf("the other account still claims the session: %+v", users[0])
	}

	// An empty AutoLoginUser write removes the value, which is what makes Steam
	// show the chooser instead of signing the previous account back in.
	if _, _, err := winutil.RegistryRead(steamTestRegBase + ":AutoLoginUser"); err == nil {
		t.Error("AutoLoginUser should be cleared when no login name is known")
	}
}

// "Add New" against an account-less install leaves a zero-byte file behind.
// That parses to no users rather than an error, so a later switch must still
// be able to rebuild the row.
func TestWriteLoginUsersAndRegistry_RebuildsEmptyFile(t *testing.T) {
	seedStoredAccount(t, accountstore.Record{
		SteamID64:   "76561198000000100",
		AccountName: "player1",
		Source:      accountstore.SourceVDF,
	})

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	loginPath := filepath.Join(configDir, "loginusers.vdf")
	os.WriteFile(loginPath, nil, 0o644)

	if err := writeLoginUsersAndRegistry(dir, "76561198000000100"); err != nil {
		t.Fatalf("writeLoginUsersAndRegistry: %v", err)
	}
	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	if len(users) != 1 || users[0].AccountName != "player1" {
		t.Fatalf("got %+v, want the single re-created account", users)
	}
}

// An unreadable file is not the same as an absent one: overwriting it would
// destroy whatever accounts it still holds.
func TestWriteLoginUsersAndRegistry_KeepsUnreadableFile(t *testing.T) {
	seedStoredAccount(t, accountstore.Record{
		SteamID64:   "76561198000000100",
		AccountName: "player1",
		Source:      accountstore.SourceVDF,
	})

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	loginPath := filepath.Join(configDir, "loginusers.vdf")
	const body = `"users" { "76561198000000100" { "AccountName"`
	os.WriteFile(loginPath, []byte(body), 0o644)

	if err := writeLoginUsersAndRegistry(dir, "76561198000000100"); err == nil {
		t.Fatal("a corrupt loginusers.vdf should abort the switch, not be replaced")
	}
	raw, err := os.ReadFile(loginPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatalf("corrupt loginusers.vdf was rewritten:\n%s", raw)
	}
}

func TestWriteLoginUsersAndRegistry_UnknownAccountFails(t *testing.T) {
	paths.ResetForTest(t.TempDir())

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "config"), 0o755)

	if err := writeLoginUsersAndRegistry(dir, "76561198000000999"); err == nil {
		t.Fatal("switching to an account neither Steam nor the switcher knows should fail")
	}
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

// Forgetting a store-only account reaches a file that has no such row. That is
// not an error, and it must not churn the backup either.
func TestRemoveSteamAccountFromVDF_NoMatchLeavesFileAlone(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)

	loginPath := filepath.Join(configDir, "loginusers.vdf")
	body := `"users"
{
	"76561198000000100"
	{
		"AccountName"		"keep"
		"PersonaName"		"Keep"
		"Timestamp"		"1"
	}
}
`
	os.WriteFile(loginPath, []byte(body), 0o644)

	if err := RemoveSteamAccountFromVDF(dir, "76561198000000999"); err != nil {
		t.Fatalf("RemoveSteamAccountFromVDF: %v", err)
	}
	raw, err := os.ReadFile(loginPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Errorf("file was rewritten for a no-op removal:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join(configDir, "loginusers.vdf_last")); err == nil {
		t.Error("a no-op removal should not create a backup")
	}
}

func TestRemoveSteamAccountFromVDF_MissingFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "config"), 0o755)
	if err := RemoveSteamAccountFromVDF(dir, "76561198000000100"); err != nil {
		t.Fatalf("removing from a deleted loginusers.vdf: %v", err)
	}
}
