package steamguard

import (
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// seedSessionTokens rewrites the fixture account's stored session. It rotates the
// vault generation, so callers must take a fresh grant afterwards.
func seedSessionTokens(t *testing.T, service *Service, accountID, accessToken, refreshToken string) {
	t.Helper()
	account := authServiceAccount(t, service.vault)
	account.Session = &mafile.SessionData{
		SteamID: authServiceSteamID, AccessToken: accessToken, RefreshToken: refreshToken,
	}
	raw, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(raw)
	if _, err := service.vault.PutRecord(accountID, raw); err != nil {
		t.Fatal(err)
	}
}

// TestEnsureFreshSessionLeavesALiveSessionAlone guards the reason renewal is not
// simply done everywhere: every vault write rotates the generation and invalidates
// the caller's capability. A session that still works must cost neither.
func TestEnsureFreshSessionLeavesALiveSessionAlone(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)
	seedSessionTokens(t, service, accountID,
		accessTokenExpiringAt(time.Now().Add(time.Hour)), "old-refresh-token")
	grant := issueSensitiveGrant(t, service, accountID, "request-fresh-live")

	renewed := false
	service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		renewed = true
		return sessionrefresh.New(&authServiceTokenClient{}, v)
	}

	before := service.vault.Generation()
	state, err := service.EnsureFreshSession(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("EnsureFreshSession on a live session: %v", err)
	}
	if renewed {
		t.Fatal("a live session was renewed; the write invalidates the caller's capability for nothing")
	}
	if state.NeedsLogin {
		t.Fatalf("live session reported as needing a login, reason %q", state.Reason)
	}
	if state.CapabilityRefreshRequired {
		t.Fatal("nothing was written, so the capability must stay valid")
	}
	if after := service.vault.Generation(); after != before {
		t.Fatal("a live session rotated the vault generation")
	}
}

// TestEnsureFreshSessionRenewsALapsedSession is the whole point of the method: an
// access token expires in about a day, the refresh token beside it lasts months,
// and only the second one running out should cost the user a password.
func TestEnsureFreshSessionRenewsALapsedSession(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)
	seedSessionTokens(t, service, accountID,
		accessTokenExpiringAt(time.Now().Add(-time.Hour)), "old-refresh-token")
	grant := issueSensitiveGrant(t, service, accountID, "request-fresh-lapsed")

	client := &authServiceTokenClient{result: protocol.TokenResult{
		State:        protocol.AuthResultTokenIssued,
		AccessToken:  accessTokenExpiringAt(time.Now().Add(time.Hour)),
		RefreshToken: "renewed-refresh-token",
	}}
	service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		return sessionrefresh.New(client, v)
	}

	before := service.vault.Generation()
	state, err := service.EnsureFreshSession(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("EnsureFreshSession on a lapsed session: %v", err)
	}
	if state.NeedsLogin {
		t.Fatalf("a lapsed access token with a usable refresh token asked for a login, reason %q", state.Reason)
	}
	if !state.CapabilityRefreshRequired {
		t.Fatal("the renewal wrote to the vault but did not tell the caller to re-acquire its capability")
	}
	if after := service.vault.Generation(); after == before {
		t.Fatal("no renewal was written; this test no longer covers what it claims")
	}
}

// TestEnsureFreshSessionWithoutARefreshTokenAsksForALogin covers the one state no
// renewal can recover from - and pins that it is answered locally, without asking
// Steam to reject a token that was never stored.
func TestEnsureFreshSessionWithoutARefreshTokenAsksForALogin(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)
	seedSessionTokens(t, service, accountID, accessTokenExpiringAt(time.Now().Add(-time.Hour)), "")
	grant := issueSensitiveGrant(t, service, accountID, "request-fresh-norefresh")

	contacted := false
	service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		contacted = true
		return sessionrefresh.New(&authServiceTokenClient{}, v)
	}

	before := service.vault.Generation()
	state, err := service.EnsureFreshSession(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("EnsureFreshSession without a refresh token: %v", err)
	}
	if contacted {
		t.Fatal("there is nothing to renew from, so Steam must not be contacted")
	}
	if !state.NeedsLogin || state.Reason != "token_expired" {
		t.Fatalf("state = %+v, want NeedsLogin with reason token_expired", state)
	}
	if after := service.vault.Generation(); after != before {
		t.Fatal("a read-only verdict rotated the vault generation")
	}
}
