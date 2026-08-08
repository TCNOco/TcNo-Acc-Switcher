package steamguard

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/loginrecord"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// seedLoginOnlyRecordWithoutRefresh stores a session that cannot be renewed, the
// one state no refresh recovers from.
func seedLoginOnlyRecordWithoutRefresh(
	t *testing.T,
	v *vault.Vault,
	steamID uint64,
	accountName, accessToken string,
) string {
	t.Helper()
	raw, err := loginrecord.Encode(loginrecord.New(steamID, accountName, accessToken, ""))
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

func promotionEnrollmentManager() *authServiceEnrollmentManager {
	return &authServiceEnrollmentManager{startStatus: enrollmentflow.Status{
		State: enrollmentapi.StateAwaitingSMS, Confirmation: enrollmentapi.ConfirmationSMS,
		Pending: true, RevocationViewAvailable: true,
	}}
}

// The point of the promotion: the record already holds a session, so enrollment
// starts from it and the user is never asked for a password.
func TestPromoteLoginOnlyAccountEnrollsFromTheStoredSession(t *testing.T) {
	service, anchorID, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecordWithToken(t, service.vault, loginOnlySteamID, "session_only", "stored-access-token")
	grant := issueSensitiveGrant(t, service, loginID, "request-promote1")
	enrollmentManager := promotionEnrollmentManager()
	service.enrollmentManager = enrollmentManager
	var registryState registry.State
	service.registryUpsertFn = func(_ string, state registry.State) error {
		registryState = state
		return nil
	}
	_ = anchorID

	promotion, err := service.PromoteLoginOnlyAccount(loginID, grant.Capability)
	if err != nil {
		t.Fatalf("PromoteLoginOnlyAccount: %v", err)
	}
	if promotion.NeedsLogin {
		t.Fatalf("a live stored session still asked for a password: %#v", promotion)
	}
	if promotion.Enrollment == nil || !promotion.Enrollment.Pending ||
		promotion.Enrollment.State != string(enrollmentapi.StateAwaitingSMS) {
		t.Fatalf("unexpected enrollment: %#v", promotion.Enrollment)
	}
	if string(enrollmentManager.startAccessToken) != "stored-access-token" {
		t.Fatalf("enrollment used %q, want the record's stored access token", enrollmentManager.startAccessToken)
	}
	// The pending record replaced the login-only one, so every capability issued
	// against the old vault generation is stale.
	if !promotion.CapabilityRefreshRequired {
		t.Fatal("a promotion that wrote the vault did not ask for a fresh capability")
	}
	if registryState != registry.StatePending {
		t.Fatalf("registry state = %v, want pending", registryState)
	}
}

// The frontend gate is UX; this one is the rule. An authenticator's secrets
// exist nowhere else, so a promotion must never reach the enrollment call for
// one.
func TestPromoteLoginOnlyAccountRefusesAnAuthenticator(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	enrollmentManager := promotionEnrollmentManager()
	service.enrollmentManager = enrollmentManager
	service.registryUpsertFn = func(string, registry.State) error { return nil }

	if _, err := service.PromoteLoginOnlyAccount(accountID, grant.Capability); !errors.Is(err, ErrNotLoginOnly) {
		t.Fatalf("promoting an authenticator = %v, want ErrNotLoginOnly", err)
	}
	if enrollmentManager.startCalls != 0 {
		t.Fatalf("Steam was asked to add an authenticator %d times", enrollmentManager.startCalls)
	}
}

// The password route has to clear the same guard the promotion does. A
// login-only record holds the account's slot, so without this an enrollment
// started from the credential form is refused as "already enrolled" for an
// account that has no authenticator at all.
func TestCredentialEnrollmentReplacesALoginOnlyRecord(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	grant := issueSensitiveGrant(t, service, loginID, "request-promote3")
	authManager := &authServiceCredentialManager{steamID: loginOnlySteamID}
	enrollmentManager := promotionEnrollmentManager()
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return authManager, nil }
	service.enrollmentManager = enrollmentManager
	service.registryUpsertFn = func(string, registry.State) error { return nil }

	begin, err := service.BeginCredentialLogin(
		loginID, grant.Capability, "session_only", "password-marker", SteamAuthPurposeAddAuthenticator,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PollCredentialLogin(loginID, grant.Capability, begin.Handle); err != nil {
		t.Fatal(err)
	}
	if !enrollmentManager.startReplaceLoginOnly {
		t.Fatal("enrollment over a login-only record did not ask to replace it, so Start would refuse it")
	}
}

// No refresh token and a session Steam will not accept means a password, and
// that is a report rather than an error: the caller falls back to the ordinary
// sign-in form, pre-filled with the name reported here.
func TestPromoteLoginOnlyAccountReportsAnUnusableSession(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	lapsed := accessTokenExpiringAt(time.Now().Add(-time.Hour))
	loginID := seedLoginOnlyRecordWithoutRefresh(t, service.vault, loginOnlySteamID, "session_only", lapsed)
	grant := issueSensitiveGrant(t, service, loginID, "request-promote2")
	enrollmentManager := promotionEnrollmentManager()
	service.enrollmentManager = enrollmentManager
	service.registryUpsertFn = func(string, registry.State) error { return nil }

	promotion, err := service.PromoteLoginOnlyAccount(loginID, grant.Capability)
	if err != nil {
		t.Fatalf("PromoteLoginOnlyAccount: %v", err)
	}
	if !promotion.NeedsLogin || promotion.Reason != "token_expired" {
		t.Fatalf("unexpected promotion: %#v", promotion)
	}
	if promotion.AccountName != "session_only" {
		t.Fatalf("account name = %q, want the stored login name", promotion.AccountName)
	}
	if enrollmentManager.startCalls != 0 {
		t.Fatalf("enrollment started against a session Steam had refused (%d calls)", enrollmentManager.startCalls)
	}
}
