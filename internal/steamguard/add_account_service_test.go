package steamguard

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steam/accountstore"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
)

// addedSteamID is deliberately not the account newAuthServiceFixture seeds: an
// add is for an account the vault has never held.
const addedSteamID = uint64(76561198000000777)

func issuePendingGrant(t *testing.T, service *Service, pendingID, requestID string) SensitiveViewGrant {
	t.Helper()
	var grant SensitiveViewGrant
	service.emitMainWindowEventFn = func(name string, data any) error {
		if name != SensitiveViewGrantEvent {
			return nil
		}
		value, ok := data.(SensitiveViewGrant)
		if !ok {
			t.Fatalf("grant event data = %#v", data)
		}
		grant = value
		return nil
	}
	if err := service.RequestAddAccountView(pendingID, requestID); err != nil {
		t.Fatal(err)
	}
	if grant.Capability == "" || grant.AccountID != pendingID {
		t.Fatalf("invalid pending grant = %#v", grant)
	}
	return grant
}

func TestValidPendingAddFormat(t *testing.T) {
	valid := pendingAddPrefix + strings.Repeat("a", pendingAddDigits)
	for _, tc := range []struct {
		name string
		id   string
		want bool
	}{
		{"well formed", valid, true},
		{"empty", "", false},
		{"no prefix", strings.Repeat("a", pendingAddDigits), false},
		{"short", pendingAddPrefix + strings.Repeat("a", pendingAddDigits-1), false},
		{"long", pendingAddPrefix + strings.Repeat("a", pendingAddDigits+1), false},
		{"non hex", pendingAddPrefix + strings.Repeat("z", pendingAddDigits), false},
		// Accepting both cases would make two spellings of one id.
		{"upper case hex", pendingAddPrefix + strings.Repeat("A", pendingAddDigits), false},
		{"a real steam id", strconv.FormatUint(addedSteamID, 10), false},
	} {
		if got := validPendingAddFormat(tc.id); got != tc.want {
			t.Errorf("%s: validPendingAddFormat(%q) = %v, want %v", tc.name, tc.id, got, tc.want)
		}
	}
}

// The shape check is not the gate. An id this service never issued must be
// refused however well formed it looks.
func TestPendingAddMustHaveBeenIssued(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)

	fabricated := pendingAddPrefix + strings.Repeat("ab", pendingAddDigits/2)
	if !validPendingAddFormat(fabricated) {
		t.Fatal("test id is not well formed")
	}
	if service.validPendingAdd(fabricated) {
		t.Fatal("a fabricated pending id was accepted")
	}

	issued, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	if !validPendingAddFormat(issued) {
		t.Fatalf("issued id %q is malformed", issued)
	}
	if !service.validPendingAdd(issued) {
		t.Fatal("an issued pending id was rejected")
	}
}

func TestPendingAddAttemptsAreDistinctAndExpire(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)

	first, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	// Distinct ids are what keep two concurrent adds off each other's busy lock
	// and authflow slot.
	if first == second {
		t.Fatal("two attempts share an id")
	}

	service.pendingAddMu.Lock()
	service.pendingAdds[first] = pendingAdd{issued: time.Now().Add(-pendingAddTTL - time.Second)}
	service.pendingAddMu.Unlock()

	if service.validPendingAdd(first) {
		t.Error("an expired attempt was accepted")
	}
	if !service.validPendingAdd(second) {
		t.Error("an unexpired attempt was dropped with the expired one")
	}
}

func TestConsumePendingAddPreventsReplay(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	id, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	service.consumePendingAdd(id)
	if service.validPendingAdd(id) {
		t.Fatal("a consumed pending id was accepted again")
	}
}

// Pins the boundary the whole design rests on: the strict path must not have
// been loosened to let an add through it.
func TestStrictPathStillRejectsPendingIdentities(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	id, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := canonicalSteamID(id); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("canonicalSteamID(%q) err = %v, want ErrAccountNotFound", id, err)
	}
	if _, _, _, err := service.authorizeSteamFlow(id, "any-token"); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("authorizeSteamFlow err = %v, want ErrAccountNotFound", err)
	}
	if err := service.RequestSensitiveView(id, "request-pending-through-strict"); !errors.Is(err, ErrSensitiveView) {
		t.Errorf("RequestSensitiveView err = %v, want ErrSensitiveView", err)
	}
	// ...and the reverse: a real SteamID64 must not reach the pending path.
	if err := service.RequestAddAccountView(strconv.FormatUint(authServiceSteamID, 10), "request-strict-through-pending"); !errors.Is(err, ErrSensitiveView) {
		t.Errorf("RequestAddAccountView with a real id err = %v, want ErrSensitiveView", err)
	}
}

func TestBeginAddAccountLoginRejectsLoginAgain(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	id, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	grant := issuePendingGrant(t, service, id, "request-add-login-again")

	// There is nothing to sign back into: an add is for an account the vault
	// does not hold.
	if _, err := service.BeginAddAccountLogin(id, grant.Capability, "new_account", "pw", SteamAuthPurposeLoginAgain); !errors.Is(err, ErrSteamAuthenticationPurpose) {
		t.Fatalf("err = %v, want ErrSteamAuthenticationPurpose", err)
	}
}

// The whole point: the account is stored under the SteamID Steam named, not
// under the pending id the flow ran on.
func TestAddAccountLoginOnlyStoresUnderTheDiscoveredSteamID(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	manager := &authServiceCredentialManager{steamID: addedSteamID}
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return manager, nil }

	var registered []string
	service.registryUpsertFn = func(id string, state registry.State) error {
		if state == registry.StateLoginOnly {
			registered = append(registered, id)
		}
		return nil
	}

	pendingID, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	grant := issuePendingGrant(t, service, pendingID, "request-add-login-only")

	begin, err := service.BeginAddAccountLogin(pendingID, grant.Capability, "new_account", "password-marker", SteamAuthPurposeLoginOnly)
	if err != nil || begin.Handle == "" {
		t.Fatalf("begin = %#v, err = %v", begin, err)
	}
	if manager.binding.ExpectedSteamID != 0 {
		t.Fatalf("binding.ExpectedSteamID = %d, want 0 so authflow accepts Steam's answer", manager.binding.ExpectedSteamID)
	}
	if manager.binding.AccountID != pendingID {
		t.Fatalf("binding.AccountID = %q, want the pending id", manager.binding.AccountID)
	}

	if _, err := service.SubmitAddAccountCode(pendingID, grant.Capability, begin.Handle, "device_code", "A1B2C"); err != nil {
		t.Fatal(err)
	}
	result, err := service.PollAddAccountLogin(pendingID, grant.Capability, begin.Handle)
	if err != nil {
		t.Fatal(err)
	}

	wantID := strconv.FormatUint(addedSteamID, 10)
	if result.SteamID64 != wantID {
		t.Fatalf("result.SteamID64 = %q, want the discovered %q", result.SteamID64, wantID)
	}
	if result.Outcome != "session_updated" {
		t.Errorf("outcome = %q, want session_updated", result.Outcome)
	}
	if len(registered) != 1 || registered[0] != wantID {
		t.Errorf("registry ids = %v, want [%s]", registered, wantID)
	}
	if _, ok, err := accountstore.Get(wantID); err != nil || !ok {
		t.Errorf("account store missing the added account: ok=%v err=%v", ok, err)
	}
	if _, ok, err := accountstore.Get(pendingID); err == nil && ok {
		t.Error("the pending id leaked into the account store")
	}

	// Spent: the same id must not start a second login.
	if service.validPendingAdd(pendingID) {
		t.Error("pending id survived a completed add")
	}
	if _, err := service.BeginAddAccountLogin(pendingID, grant.Capability, "new_account", "pw", SteamAuthPurposeLoginOnly); err == nil {
		t.Error("a spent pending id started another login")
	}
}

func TestAddAccountEnrollmentStartsUnderTheDiscoveredSteamID(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	manager := &authServiceCredentialManager{steamID: addedSteamID}
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return manager, nil }
	enrollment := &authServiceEnrollmentManager{startStatus: enrollmentflow.Status{
		State: enrollmentapi.StateAwaitingSMS, Confirmation: enrollmentapi.ConfirmationSMS,
		Pending: true, RevocationViewAvailable: true,
	}}
	service.enrollmentManager = enrollment
	service.registryUpsertFn = func(string, registry.State) error { return nil }

	pendingID, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	grant := issuePendingGrant(t, service, pendingID, "request-add-authenticator")

	begin, err := service.BeginAddAccountLogin(pendingID, grant.Capability, "new_account", "password-marker", SteamAuthPurposeAddAuthenticator)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SubmitAddAccountCode(pendingID, grant.Capability, begin.Handle, "device_code", "A1B2C"); err != nil {
		t.Fatal(err)
	}
	result, err := service.PollAddAccountLogin(pendingID, grant.Capability, begin.Handle)
	if err != nil {
		t.Fatal(err)
	}

	if result.Outcome != "enrollment_pending" {
		t.Fatalf("outcome = %q, want enrollment_pending", result.Outcome)
	}
	// The caller needs this to re-key onto the strict, SteamID-only enrollment
	// endpoints for Resume/Acknowledge/Finalize.
	if result.SteamID64 != strconv.FormatUint(addedSteamID, 10) {
		t.Fatalf("result.SteamID64 = %q, want the discovered id", result.SteamID64)
	}
	if enrollment.startCalls != 1 {
		t.Fatalf("enrollment start calls = %d, want 1", enrollment.startCalls)
	}
}

// A capability minted for one attempt must not authorise another.
func TestAddAccountRejectsAForeignCapability(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	service.newAuthManager = func() (steamCredentialAuthManager, error) {
		return &authServiceCredentialManager{steamID: addedSteamID}, nil
	}

	first, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.NewAddAccountAttempt()
	if err != nil {
		t.Fatal(err)
	}
	foreign := issuePendingGrant(t, service, second, "request-add-foreign")

	if _, err := service.BeginAddAccountLogin(first, foreign.Capability, "new_account", "pw", SteamAuthPurposeLoginOnly); err == nil {
		t.Fatal("a capability bound to another attempt was accepted")
	}
}
