package steamguard

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// expiredAccessToken is a JWT-shaped token whose exp is long past. The fixture's
// placeholder token is unreadable, and an unreadable token counts as live — so a
// refresh only happens with a token the expiry check can actually read.
func expiredAccessToken() string {
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1000000000}`))
	return "eyJhbGciOiJFZERTQSJ9." + claims + ".c2ln"
}

// newBrowserSessionFixture is newAuthServiceFixture with a session that has
// actually lapsed, so opening a window has to renew it first.
func newBrowserSessionFixture(t *testing.T) (*Service, string, SensitiveViewGrant) {
	t.Helper()
	useSettingsRoot(t)
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	if _, err := service.Initialize(authServiceVaultPassword, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Unlock(authServiceVaultPassword, vault.ProcessLease); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.vault.Lock() })

	raw, err := mafile.ExportPlaintext(mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x32}, 20)),
		DeviceID:       "android:00112233-4455-6677-8899-aabbccddeeff",
		AccountName:    "test_account",
		FullyEnrolled:  true,
		Session: &mafile.SessionData{
			SteamID: authServiceSteamID, AccessToken: expiredAccessToken(), RefreshToken: "old-refresh-token",
		},
	}, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(raw)

	accountID := strconv.FormatUint(authServiceSteamID, 10)
	if _, err := service.vault.PutRecord(accountID, raw); err != nil {
		t.Fatal(err)
	}
	// After the write, so the grant is bound to the generation the test starts from.
	grant := issueSensitiveGrant(t, service, accountID, "request-browser-session1")

	service.registryUpsertFn = func(string, registry.State) error { return nil }
	service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		return sessionrefresh.New(&authServiceTokenClient{result: protocol.TokenResult{
			State:        protocol.AuthResultTokenIssued,
			AccessToken:  "refreshed-access-token",
			RefreshToken: "refreshed-refresh-token",
		}}, v)
	}
	return service, accountID, grant
}

// Opening a browser window renews a stale session first, and that renewal writes
// to the vault, which rotates the generation and invalidates the capability the
// window was opened with. Authorizing a second time inside the same call would
// reject the caller with its own side effect.
func TestBrowserSessionSurvivesItsOwnSessionRefresh(t *testing.T) {
	service, accountID, grant := newBrowserSessionFixture(t)

	before := service.vault.Generation()
	session, err := NewBrowserSessionSource(service).BrowserSession(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("browser session failed after renewing its own session: %v", err)
	}
	if service.vault.Generation() == before {
		t.Fatal("nothing was written, so this test no longer covers the case it claims")
	}
	if session.AccessToken != "refreshed-access-token" {
		t.Fatalf("window opened with a stale token: %q", session.AccessToken)
	}
	if !session.CapabilityRefreshRequired {
		t.Fatal("the vault write was not reported, so the modal keeps a stale capability")
	}
	if session.SessionID == "" || session.AccountName == "" {
		t.Fatalf("incomplete session: %+v", session)
	}
}

// The second window: the token the first one stored is live, so nothing is
// renewed, nothing is written, and the capability the caller arrives with stays
// valid. This also walks the contract the UI follows — re-acquire once the
// previous call reported a write — so the two halves are pinned together.
func TestBrowserSessionLeavesALiveSessionAlone(t *testing.T) {
	service, accountID, grant := newBrowserSessionFixture(t)
	// Renew once so the stored token is the live one the second call will find.
	first, err := NewBrowserSessionSource(service).BrowserSession(accountID, grant.Capability)
	if err != nil {
		t.Fatal(err)
	}
	if !first.CapabilityRefreshRequired {
		t.Fatal("the renewal was not reported, so the UI would never re-acquire")
	}
	grant = issueSensitiveGrant(t, service, accountID, "request-browser-session2")

	before := service.vault.Generation()
	session, err := NewBrowserSessionSource(service).BrowserSession(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("second window failed: %v", err)
	}
	if service.vault.Generation() != before {
		t.Fatal("a live session was renewed anyway, rotating the generation for nothing")
	}
	if session.CapabilityRefreshRequired {
		t.Fatal("nothing was written, so no capability refresh should be asked for")
	}
}
