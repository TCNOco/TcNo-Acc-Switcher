package steamguard

import (
	"encoding/base64"
	"errors"
	"strconv"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/loginrecord"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

const loginOnlySteamID = uint64(76561198000000123)

func seedLoginOnlyRecord(t *testing.T, v *vault.Vault, steamID uint64, accountName string) string {
	t.Helper()
	return seedLoginOnlyRecordWithToken(t, v, steamID, accountName, "access-token-for-tests")
}

func seedLoginOnlyRecordWithToken(
	t *testing.T,
	v *vault.Vault,
	steamID uint64,
	accountName, accessToken string,
) string {
	t.Helper()
	return seedLoginOnlyRecordWithTokens(t, v, steamID, accountName, accessToken, "refresh-token-for-tests")
}

func seedLoginOnlyRecordWithTokens(
	t *testing.T,
	v *vault.Vault,
	steamID uint64,
	accountName, accessToken, refreshToken string,
) string {
	t.Helper()
	record := loginrecord.New(steamID, accountName, accessToken, refreshToken)
	raw, err := loginrecord.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(raw)
	steamID64 := strconv.FormatUint(steamID, 10)
	if _, err := v.PutRecord(steamID64, raw); err != nil {
		t.Fatal(err)
	}
	return steamID64
}

func TestListAccountsReturnsEveryRecordShape(t *testing.T) {
	// A single login-only or half-finished record must not hide every other
	// account in the picker.
	service, anchorID, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	// Seeding wrote the vault, which rotates its generation and invalidates the
	// fixture's capability, so take a fresh one.
	grant := issueSensitiveGrant(t, service, anchorID, "request-login-only-list")

	summaries, err := service.ListAccounts(anchorID, grant.Capability)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("summaries = %#v, want 2", summaries)
	}
	byID := make(map[string]AccountSummary, len(summaries))
	for _, summary := range summaries {
		byID[summary.SteamID64] = summary
	}
	if got := byID[anchorID]; got.Kind != AccountKindAuthenticator || got.AccountName != "test_account" {
		t.Fatalf("authenticator summary = %#v", got)
	}
	if got := byID[loginID]; got.Kind != AccountKindLoginOnly || got.AccountName != "session_only" {
		t.Fatalf("login-only summary = %#v", got)
	}
	// Neither fixture token is a JWT, so neither carries an expiry to read. An
	// unreadable token must not be reported as lapsed - the picker would send the
	// user to a sign-in that nothing said was needed.
	for id, summary := range byID {
		if summary.SessionStatus != SessionStatusUnknown {
			t.Fatalf("summary %s status = %q, want unknown", id, summary.SessionStatus)
		}
	}
}

// The picker decides this for every row when the list is drawn, so an account
// whose session has lapsed is visible without opening it.
func TestListAccountsReportsSessionStatusPerRow(t *testing.T) {
	service, anchorID, _ := newAuthServiceFixture(t)
	// Both tokens gone: the only row here that genuinely needs a password.
	expiredID := seedLoginOnlyRecordWithTokens(t, service.vault, loginOnlySteamID, "expired_session",
		accessTokenExpiringAt(time.Now().Add(-time.Hour)), accessTokenExpiringAt(time.Now().Add(-time.Hour)))
	liveID := seedLoginOnlyRecordWithToken(t, service.vault, loginOnlySteamID+1, "live_session",
		accessTokenExpiringAt(time.Now().Add(24*time.Hour)))
	// The ordinary overnight case: the access token aged out, the refresh token
	// beside it has months left, and opening the row renews it with no password.
	renewableID := seedLoginOnlyRecordWithTokens(t, service.vault, loginOnlySteamID+2, "renewable_session",
		accessTokenExpiringAt(time.Now().Add(-time.Hour)), accessTokenExpiringAt(time.Now().Add(200*24*time.Hour)))
	grant := issueSensitiveGrant(t, service, anchorID, "request-session-status-list")

	summaries, err := service.ListAccounts(anchorID, grant.Capability)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	byID := make(map[string]AccountSummary, len(summaries))
	for _, summary := range summaries {
		byID[summary.SteamID64] = summary
	}
	// A login-only record cannot hold an empty access token (loginrecord rejects
	// one), so the no-session case is covered against the classifier instead.
	for id, want := range map[string]SessionStatus{
		expiredID:   SessionStatusNeedsLogin,
		liveID:      SessionStatusValid,
		renewableID: SessionStatusValid,
	} {
		if got := byID[id].SessionStatus; got != want {
			t.Fatalf("summary %s status = %q, want %q", id, got, want)
		}
	}
}

// Built the way sessionrefresh's own tests build one: only the payload's exp is read.
func accessTokenExpiringAt(expiry time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"exp":` + strconv.FormatInt(expiry.Unix(), 10) + `}`))
	return "header." + payload + ".signature"
}

func TestCodeAndExportDeclineALoginOnlyAccount(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	grant := issueSensitiveGrant(t, service, loginID, "request-login-only-1")

	if _, err := service.GetCode(loginID, grant.Capability); !errors.Is(err, ErrNotAuthenticator) {
		t.Fatalf("GetCode err = %v, want ErrNotAuthenticator", err)
	}
	records, err := service.vault.ListRecords()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.SteamID64 != loginID {
			continue
		}
		if _, err := accountFromRecord(service.vault, record.ID); !errors.Is(err, ErrNotAuthenticator) {
			t.Fatalf("accountFromRecord err = %v, want ErrNotAuthenticator", err)
		}
	}
}

func TestPutLoginOnlyRecordRefusesToReplaceAnAuthenticator(t *testing.T) {
	// PutRecord replaces by SteamID64, so without this guard one mis-click would
	// destroy a shared secret, an identity secret and a revocation code - none of
	// which can be recovered.
	service, accountID, _ := newAuthServiceFixture(t)
	steamID, err := canonicalSteamID(accountID)
	if err != nil {
		t.Fatal(err)
	}
	err = putLoginOnlyRecord(service.vault, steamID, []byte("someone"), []byte("access-token-x"), []byte("refresh-token-x"))
	if !errors.Is(err, ErrWouldReplaceAuthenticator) {
		t.Fatalf("err = %v, want ErrWouldReplaceAuthenticator", err)
	}
	if _, err := accountFromRecord(service.vault, mustRecordID(t, service.vault, accountID)); err != nil {
		t.Fatalf("authenticator no longer readable: %v", err)
	}
}

func TestPutLoginOnlyRecordReplacesAnExistingLoginOnlyRecord(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "before")

	if err := putLoginOnlyRecord(service.vault, loginOnlySteamID,
		[]byte("after"), []byte("access-token-new"), []byte("refresh-token-new")); err != nil {
		t.Fatalf("putLoginOnlyRecord: %v", err)
	}
	loaded, err := recordForSteamID64(service.vault, strconv.FormatUint(loginOnlySteamID, 10))
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.destroy()
	if loaded.Kind != vaultrecord.KindLoginOnly || loaded.AccountName() != "after" || loaded.AccessToken() != "access-token-new" {
		t.Fatalf("record = %#v", loaded)
	}
}

func TestRemoveLoginOnlyAccountRefusesAnAuthenticator(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	if _, err := service.RemoveLoginOnlyAccount(accountID, grant.Capability); !errors.Is(err, ErrNotAuthenticator) {
		t.Fatalf("err = %v, want ErrNotAuthenticator", err)
	}
	if _, err := accountFromRecord(service.vault, mustRecordID(t, service.vault, accountID)); err != nil {
		t.Fatalf("authenticator was removed: %v", err)
	}
}

func TestRemoveLoginOnlyAccountDeletesTheRecordAndRegistryEntry(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	if err := registry.Upsert(loginID, registry.StateLoginOnly); err != nil {
		t.Fatal(err)
	}
	grant := issueSensitiveGrant(t, service, loginID, "request-login-only-2")

	result, err := service.RemoveLoginOnlyAccount(loginID, grant.Capability)
	if err != nil {
		t.Fatalf("RemoveLoginOnlyAccount: %v", err)
	}
	if !result.CapabilityRefreshRequired {
		t.Fatal("CapabilityRefreshRequired = false; DeleteRecord rotates the generation")
	}
	if _, err := recordForSteamID64(service.vault, loginID); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("record still present: %v", err)
	}
	entries, err := registry.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.SteamID64 == loginID {
			t.Fatal("registry entry survived removal")
		}
	}
}

func mustRecordID(t *testing.T, v *vault.Vault, steamID64 string) string {
	t.Helper()
	records, err := v.ListRecords()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.SteamID64 == steamID64 {
			return record.ID
		}
	}
	t.Fatalf("no record for %s", steamID64)
	return ""
}

// The switcher's Forget reaches this in-process, with no capability token, so
// what it may delete is decided here rather than by whoever called it.
func TestForgetLoginOnlyRecordDeletesTheRecordAndRegistryEntry(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	if err := registry.Upsert(loginID, registry.StateLoginOnly); err != nil {
		t.Fatal(err)
	}

	if err := service.forgetLoginOnlyRecord(loginID); err != nil {
		t.Fatalf("forgetLoginOnlyRecord: %v", err)
	}

	if _, err := recordForSteamID64(service.vault, loginID); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("record still present: %v", err)
	}
	if registryHolds(t, loginID) {
		t.Fatal("registry entry survived the forget, so the row would come back")
	}
}

func TestForgetLoginOnlyRecordRefusesAnAuthenticator(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)

	err := service.forgetLoginOnlyRecord(accountID)

	if !errors.Is(err, steam.ErrForgetSteamGuardAuthenticator) {
		t.Fatalf("err = %v, want ErrForgetSteamGuardAuthenticator", err)
	}
	if _, err := accountFromRecord(service.vault, mustRecordID(t, service.vault, accountID)); err != nil {
		t.Fatalf("authenticator was removed: %v", err)
	}
}

// Deleting needs the keys, and forgetting the row while the record survives is
// exactly the half-state this path exists to prevent.
func TestForgetLoginOnlyRecordRefusesALockedVault(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	if err := registry.Upsert(loginID, registry.StateLoginOnly); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}

	err := service.forgetLoginOnlyRecord(loginID)

	if !errors.Is(err, steam.ErrForgetSteamGuardLocked) {
		t.Fatalf("err = %v, want ErrForgetSteamGuardLocked", err)
	}
	if !registryHolds(t, loginID) {
		t.Fatal("registry entry cleared while its record survives")
	}
}

// An index entry with no record behind it is enough on its own to rebuild the
// account's row, so clearing it is the whole job.
func TestForgetLoginOnlyRecordClearsAStaleIndexEntry(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	staleID := strconv.FormatUint(loginOnlySteamID, 10)
	if err := registry.Upsert(staleID, registry.StateLoginOnly); err != nil {
		t.Fatal(err)
	}

	if err := service.forgetLoginOnlyRecord(staleID); err != nil {
		t.Fatalf("forgetLoginOnlyRecord: %v", err)
	}

	if registryHolds(t, staleID) {
		t.Fatal("stale registry entry survived the forget")
	}
}

func registryHolds(t *testing.T, steamID64 string) bool {
	t.Helper()
	entries, err := registry.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.SteamID64 == steamID64 {
			return true
		}
	}
	return false
}
