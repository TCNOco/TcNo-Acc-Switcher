package steam

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steam/accountstore"
	steamguardregistry "TcNo-Acc-Switcher/internal/steamguard/registry"
)

const (
	knownIDA = "76561198000000100"
	knownIDB = "76561198000000200"
)

func useTempAccountStore(t *testing.T) {
	t.Helper()
	paths.ResetForTest(t.TempDir())
}

func findUser(users []LoginUser, steamID64 string) (LoginUser, bool) {
	for _, u := range users {
		if u.SteamID64 == steamID64 {
			return u, true
		}
	}
	return LoginUser{}, false
}

func TestSyncKnownAccountsImportsVDFRows(t *testing.T) {
	useTempAccountStore(t)

	users := []LoginUser{{
		SteamID64:   knownIDA,
		AccountName: "acct_a",
		PersonaName: "Persona A",
		Timestamp:   "1700000000",
		AutoLogin:   "1",
	}}
	got := syncKnownAccounts(users)
	if len(got) != 1 || got[0].SteamID64 != knownIDA {
		t.Fatalf("got %+v, want the single vdf row", got)
	}

	rec, ok, err := accountstore.Get(knownIDA)
	if err != nil || !ok {
		t.Fatalf("account was not imported: ok=%v err=%v", ok, err)
	}
	if rec.AccountName != "acct_a" || rec.PersonaName != "Persona A" || rec.Timestamp != "1700000000" {
		t.Errorf("imported record = %+v, want the vdf fields carried over", rec)
	}
	if rec.Source != accountstore.SourceVDF {
		t.Errorf("Source = %q, want %q", rec.Source, accountstore.SourceVDF)
	}
}

// The headline case: Advanced Clearing (or Steam itself) empties the file and
// the switcher must still list every account.
func TestSyncKnownAccountsKeepsAccountsSteamForgot(t *testing.T) {
	useTempAccountStore(t)

	syncKnownAccounts([]LoginUser{
		{SteamID64: knownIDA, AccountName: "acct_a", PersonaName: "Persona A", Timestamp: "1700000000"},
		{SteamID64: knownIDB, AccountName: "acct_b", PersonaName: "Persona B"},
	})

	got := syncKnownAccounts(nil)
	if len(got) != 2 {
		t.Fatalf("got %d accounts from an empty loginusers.vdf, want 2", len(got))
	}
	a, ok := findUser(got, knownIDA)
	if !ok {
		t.Fatalf("account %s missing from %+v", knownIDA, got)
	}
	if a.AccountName != "acct_a" || a.PersonaName != "Persona A" {
		t.Errorf("restored row = %+v, want the stored names", a)
	}
	if a.Timestamp != "1700000000" {
		t.Errorf("Timestamp = %q, want it restored so last-login still renders", a.Timestamp)
	}
	if a.AutoLogin != "0" || a.MostRecent != "0" {
		t.Errorf("restored row claims a live session: AutoLogin=%q MostRecent=%q", a.AutoLogin, a.MostRecent)
	}
}

// Restored rows must not make the live account ambiguous - ActiveSessionSteamID64
// returns "" when more than one row claims the session.
func TestSyncKnownAccountsPreservesActiveSession(t *testing.T) {
	useTempAccountStore(t)

	syncKnownAccounts([]LoginUser{
		{SteamID64: knownIDA, AccountName: "acct_a"},
		{SteamID64: knownIDB, AccountName: "acct_b"},
	})

	live := []LoginUser{{SteamID64: knownIDA, AccountName: "acct_a", AutoLogin: "1"}}
	got := syncKnownAccounts(live)
	if len(got) != 2 {
		t.Fatalf("got %d accounts, want the live row plus the stored one", len(got))
	}
	if active := ActiveSessionSteamID64(got); active != knownIDA {
		t.Errorf("ActiveSessionSteamID64 = %q, want %q", active, knownIDA)
	}
}

func TestSyncKnownAccountsDeduplicates(t *testing.T) {
	useTempAccountStore(t)
	syncKnownAccounts([]LoginUser{{SteamID64: knownIDA, AccountName: "acct_a"}})

	got := syncKnownAccounts([]LoginUser{{SteamID64: knownIDA, AccountName: "acct_a"}})
	if len(got) != 1 {
		t.Fatalf("got %d rows for one account, want 1", len(got))
	}
}

func TestSyncKnownAccountsLeavesCorruptStoreAlone(t *testing.T) {
	useTempAccountStore(t)

	path, err := accountstore.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const body = `{"version":1,"entries":[ broken`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	users := []LoginUser{{SteamID64: knownIDA, AccountName: "acct_a"}}
	got := syncKnownAccounts(users)
	if len(got) != 1 || got[0].SteamID64 != knownIDA {
		t.Fatalf("got %+v, want the vdf rows passed through untouched", got)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatalf("a corrupt store was overwritten:\n%s", raw)
	}
}

func TestKnownAccountAsLoginUser(t *testing.T) {
	useTempAccountStore(t)
	syncKnownAccounts([]LoginUser{{SteamID64: knownIDA, AccountName: "acct_a", PersonaName: "Persona A"}})

	u, ok := knownAccountAsLoginUser(knownIDA)
	if !ok {
		t.Fatal("stored account should be materialisable")
	}
	if u.AccountName != "acct_a" {
		t.Errorf("AccountName = %q, want acct_a", u.AccountName)
	}
	if _, ok := knownAccountAsLoginUser(knownIDB); ok {
		t.Error("an account the switcher has never seen should not materialise")
	}
}

// steamTestEnv scaffolds the exe dir, data root and a fake Steam install so the
// service methods that resolve the install folder can run.
// Do NOT call t.Parallel() in tests using this - it sets global path singletons.
type steamTestEnv struct {
	exeDir   string
	steamDir string
}

func newSteamTestEnv(t *testing.T) *steamTestEnv {
	t.Helper()
	exeDir := t.TempDir()
	steamDir := t.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
	if err := os.MkdirAll(filepath.Join(steamDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	// main injects the embedded copy at startup, so under test nothing seeds
	// this and LoadPlatformsJSON would fail before the Steam root is resolved.
	userData := platform.UserDataDir(exeDir)
	if err := os.MkdirAll(userData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, "Platforms.json"),
		[]byte(`{"Version":"test","Platforms":{"Steam":{"ExeLocationDefault":[]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	st.FolderPath = steamDir
	if err := SaveSettings(st); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	return &steamTestEnv{exeDir: exeDir, steamDir: steamDir}
}

func (e *steamTestEnv) writeLoginUsers(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(e.steamDir, "config", "loginusers.vdf")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const oneAccountVDF = `"users"
{
	"76561198000000100"
	{
		"AccountName"		"acct_a"
		"PersonaName"		"Persona A"
		"Timestamp"		"1700000000"
	}
}
`

// Reordering validated against loginusers.vdf alone: the count can never match
// once an account lives only in the store, so every reorder was rejected.
func TestSaveSteamAccountOrderAcceptsStoreOnlyAccounts(t *testing.T) {
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)

	if _, err := accountstore.Upsert(accountstore.Record{
		SteamID64:   knownIDB,
		AccountName: "vault_only",
		Source:      accountstore.SourceSteamGuard,
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewSteamService()
	if err := svc.SaveSteamAccountOrder([]string{knownIDB, knownIDA}); err != nil {
		t.Fatalf("SaveSteamAccountOrder: %v", err)
	}

	order, err := LoadOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != knownIDB || order[1] != knownIDA {
		t.Fatalf("order = %v, want [%s %s]", order, knownIDB, knownIDA)
	}
}

func TestCountSavedAccountsIncludesStoreOnlyAccounts(t *testing.T) {
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)

	if _, err := accountstore.Upsert(accountstore.Record{
		SteamID64:   knownIDB,
		AccountName: "vault_only",
		Source:      accountstore.SourceSteamGuard,
	}); err != nil {
		t.Fatal(err)
	}
	if got := CountSavedAccounts(); got != 2 {
		t.Fatalf("CountSavedAccounts = %d, want 2", got)
	}
}

// Forget is the only way an account leaves. If it cleared the vdf row but not
// the store, the account would reappear on the next refresh.
func TestForgetSteamAccountRemovesItFromTheStoreAndOrder(t *testing.T) {
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)

	svc := NewSteamService()
	if _, err := accountstore.Upsert(accountstore.Record{
		SteamID64:   knownIDB,
		AccountName: "vault_only",
		Source:      accountstore.SourceSteamGuard,
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveOrder([]string{knownIDB, knownIDA}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ForgetSteamAccount(knownIDB); err != nil {
		t.Fatalf("ForgetSteamAccount: %v", err)
	}
	stopRefreshTimer(svc)

	if _, ok, err := accountstore.Get(knownIDB); err != nil || ok {
		t.Fatalf("forgotten account still stored: ok=%v err=%v", ok, err)
	}
	remaining := knownAccountsForRoot(env.steamDir)
	if len(remaining) != 1 || remaining[0].SteamID64 != knownIDA {
		t.Fatalf("remaining = %+v, want only %s", remaining, knownIDA)
	}
	order, err := LoadOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 || order[0] != knownIDA {
		t.Fatalf("order = %v, want the forgotten id pruned", order)
	}
}

// The whole point of the store, asserted through the call the UI actually
// makes: a loginusers.vdf the parser cannot read must cost the accounts Steam
// still knew about, not the entire list.
func TestGetSteamAccountsListSurvivesABrokenLoginUsers(t *testing.T) {
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)

	svc := NewSteamService()
	if _, err := svc.GetSteamAccountsList(); err != nil {
		t.Fatalf("seeding list build: %v", err)
	}

	env.writeLoginUsers(t, `"users" { "76561198000000100" { "AccountName"`)
	list, err := svc.GetSteamAccountsList()
	if err != nil {
		t.Fatalf("GetSteamAccountsList after corruption: %v", err)
	}
	if len(list) != 1 || list[0].SteamID64 != knownIDA {
		t.Fatalf("got %+v, want the account restored from the store", list)
	}
	if list[0].AccountName != "acct_a" || list[0].PersonaName != "Persona A" {
		t.Errorf("restored row lost its names: %+v", list[0])
	}
	if list[0].CurrentSession {
		t.Error("a store-only row must not claim to be the live session")
	}
}

// Advanced Clearing deletes the file rather than corrupting it.
func TestGetSteamAccountsListSurvivesADeletedLoginUsers(t *testing.T) {
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)

	svc := NewSteamService()
	if _, err := svc.GetSteamAccountsList(); err != nil {
		t.Fatalf("seeding list build: %v", err)
	}

	if err := os.Remove(filepath.Join(env.steamDir, "config", "loginusers.vdf")); err != nil {
		t.Fatal(err)
	}
	list, err := svc.GetSteamAccountsList()
	if err != nil {
		t.Fatalf("GetSteamAccountsList after deletion: %v", err)
	}
	if len(list) != 1 || list[0].SteamID64 != knownIDA {
		t.Fatalf("got %+v, want the account restored from the store", list)
	}
}

// Steam's own rows come first and keep their order; the ones only the switcher
// remembers follow. MergeOrder runs after this and can still reorder the lot.
func TestSyncKnownAccountsPutsVDFRowsFirst(t *testing.T) {
	useTempAccountStore(t)
	syncKnownAccounts([]LoginUser{
		{SteamID64: knownIDA, AccountName: "acct_a"},
		{SteamID64: knownIDB, AccountName: "acct_b"},
	})

	got := syncKnownAccounts([]LoginUser{{SteamID64: knownIDB, AccountName: "acct_b"}})
	if len(got) != 2 {
		t.Fatalf("got %d accounts, want 2", len(got))
	}
	if got[0].SteamID64 != knownIDB {
		t.Errorf("got[0] = %s, want the loginusers.vdf row %s first", got[0].SteamID64, knownIDB)
	}
	if got[1].SteamID64 != knownIDA {
		t.Errorf("got[1] = %s, want the store-only row %s after", got[1].SteamID64, knownIDA)
	}
}

// Restoring a SteamGuard folder (or swapping one in by hand) puts accounts in
// the vault and its registration index without going through any of the code
// paths that seed the account store. Those accounts have to reach the list too,
// or the restore looks like it silently dropped them.
func TestGetSteamAccountsListIncludesRestoredVaultAccounts(t *testing.T) {
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)

	root, err := steamguardregistry.RootPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := steamguardregistry.Upsert(knownIDB, steamguardregistry.StateActive); err != nil {
		t.Fatal(err)
	}

	svc := NewSteamService()
	list, err := svc.GetSteamAccountsList()
	if err != nil {
		t.Fatalf("GetSteamAccountsList: %v", err)
	}
	byID := make(map[string]SteamAccountListItemDTO, len(list))
	for _, row := range list {
		byID[row.SteamID64] = row
	}
	restored, ok := byID[knownIDB]
	if !ok {
		t.Fatalf("restored vault account missing from %+v", list)
	}
	if !restored.HasSteamGuard {
		t.Error("restored account should carry its Steam Guard state")
	}
	// And it must be persisted, so the profile refresh picks it up for a name
	// and an avatar rather than it vanishing on the next build.
	if _, stored, err := accountstore.Get(knownIDB); err != nil || !stored {
		t.Errorf("restored account was not imported into the store: ok=%v err=%v", stored, err)
	}
}

// An account that arrives behind the app's back has no name and no avatar, and
// nothing else asks for them: the profile refresh runs on a page load or a user
// action, and a swapped-in folder is neither.
func TestSyncKnownAccountsAsksForAProfileRefreshOnlyOnDiscovery(t *testing.T) {
	useTempAccountStore(t)
	requests := 0
	RegisterProfileRefreshTrigger(func() { requests++ })
	t.Cleanup(func() { RegisterProfileRefreshTrigger(nil) })

	syncKnownAccounts([]LoginUser{{SteamID64: knownIDA, AccountName: "acct_a"}})
	if requests != 1 {
		t.Fatalf("requests = %d after first sight of an account, want 1", requests)
	}

	// Steady state: the same accounts on every window focus must not queue a
	// refresh each time.
	syncKnownAccounts([]LoginUser{{SteamID64: knownIDA, AccountName: "acct_a"}})
	if requests != 1 {
		t.Fatalf("requests = %d after an unchanged sync, want it left at 1", requests)
	}

	// A restored Steam Guard folder: new to the store, absent from the vdf.
	root, err := steamguardregistry.RootPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := steamguardregistry.Upsert(knownIDB, steamguardregistry.StateActive); err != nil {
		t.Fatal(err)
	}
	syncKnownAccounts([]LoginUser{{SteamID64: knownIDA, AccountName: "acct_a"}})
	if requests != 2 {
		t.Fatalf("requests = %d after a restored account appeared, want 2", requests)
	}
}
