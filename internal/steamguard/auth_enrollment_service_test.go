package steamguard

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/authflow"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const authServiceSteamID = uint64(76561198000000031)

type authServiceTokenClient struct {
	calls  int
	result protocol.TokenResult
	err    error
}

func (f *authServiceTokenClient) GenerateAccessTokenForApp(context.Context, protocol.GenerateAccessTokenRequest, time.Duration) (protocol.TokenResult, error) {
	f.calls++
	return f.result, f.err
}

type authServiceCredentialManager struct {
	beginCalls   int
	beginQRCalls int
	qrURL        string
	submitCalls  int
	pollCalls    int
	consumeCalls int
	cancelCalls  int
	closed       bool
	passwordRef  []byte
	codeRef      []byte
	binding      authflow.Binding
	steamID      uint64
}

func (f *authServiceCredentialManager) Begin(_ context.Context, binding authflow.Binding, _ protocol.PasswordCredentialsRequest, password []byte) (authflow.Status, error) {
	f.beginCalls++
	f.binding = binding
	f.passwordRef = password
	return authflow.Status{
		Handle: "opaque-auth-handle", State: authflow.StateChallengeRequired,
		Challenges: []authflow.Challenge{authflow.ChallengeDeviceCode}, CanSubmitDeviceCode: true,
		ExpiresAtUnix: time.Now().Add(time.Minute).Unix(),
	}, nil
}

func (f *authServiceCredentialManager) BeginQR(_ context.Context, binding authflow.Binding, _ protocol.BeginQRRequest) (authflow.Status, error) {
	f.beginQRCalls++
	f.binding = binding
	url := f.qrURL
	if url == "" {
		url = "https://s.team/q/1/4123"
	}
	return authflow.Status{
		Handle: "opaque-qr-handle", State: authflow.StateWaiting, CanPoll: true,
		ChallengeURL:  url,
		ExpiresAtUnix: time.Now().Add(time.Minute).Unix(),
	}, nil
}

func (f *authServiceCredentialManager) SubmitCode(_ context.Context, binding authflow.Binding, _ string, _ authflow.Challenge, code []byte) (authflow.Status, error) {
	f.submitCalls++
	f.codeRef = code
	if !sameAuthBinding(f.binding, binding) {
		return authflow.Status{}, ErrSteamAuthenticationState
	}
	return authflow.Status{Handle: "opaque-auth-handle", State: authflow.StateCodeAccepted, CanPoll: true}, nil
}

func (f *authServiceCredentialManager) Poll(_ context.Context, binding authflow.Binding, _ string) (authflow.Status, error) {
	f.pollCalls++
	if !sameAuthBinding(f.binding, binding) {
		return authflow.Status{}, ErrSteamAuthenticationState
	}
	return authflow.Status{Handle: "opaque-auth-handle", State: authflow.StateAuthorizedReady}, nil
}

func (f *authServiceCredentialManager) Cancel(authflow.Binding, string) error {
	f.cancelCalls++
	return nil
}

func (f *authServiceCredentialManager) Consume(_ authflow.Binding, _ string, consumer authflow.Consumer) error {
	f.consumeCalls++
	access := []byte("credential-access-token")
	refresh := []byte("credential-refresh-token")
	defer wipe(access)
	defer wipe(refresh)
	return consumer(f.steamID, []byte("authenticated_account"), access, refresh, nil, true)
}

func (f *authServiceCredentialManager) Close() { f.closed = true }

type authServiceEnrollmentManager struct {
	startStatus           enrollmentflow.Status
	startCalls            int
	startAccessToken      []byte
	startReplaceLoginOnly bool
	resumeStatus          enrollmentflow.Status
	revealCode            string
	revealCalls           int
	ackCalls              int
	ackCodeRef            []byte
	finalizeStatuses      []enrollmentflow.Status
	finalizeErrors        []error
	finalizeCalls         int
	finalizeCodeRef       []byte
	cancelCalls           int
	mutateAck             func() error
}

func (f *authServiceEnrollmentManager) Start(_ context.Context, request enrollmentflow.StartRequest) (enrollmentflow.Status, error) {
	f.startCalls++
	f.startAccessToken = append([]byte(nil), request.AccessToken...)
	f.startReplaceLoginOnly = request.ReplaceLoginOnly
	return f.startStatus, nil
}

func (f *authServiceEnrollmentManager) Resume(uint64) (enrollmentflow.Status, error) {
	return f.resumeStatus, nil
}

func (f *authServiceEnrollmentManager) RevealRevocationCode(uint64) (enrollmentflow.RevocationView, error) {
	f.revealCalls++
	return enrollmentflow.RevocationView{Code: f.revealCode}, nil
}

func (f *authServiceEnrollmentManager) AcknowledgeRevocationCode(_ uint64, code []byte) (enrollmentflow.Status, error) {
	f.ackCalls++
	f.ackCodeRef = code
	if f.mutateAck != nil {
		if err := f.mutateAck(); err != nil {
			return enrollmentflow.Status{}, err
		}
	}
	return enrollmentflow.Status{State: enrollmentapi.StateAwaitingSMS, Pending: true}, nil
}

func (f *authServiceEnrollmentManager) Finalize(_ context.Context, request enrollmentflow.FinalizeRequest) (enrollmentflow.Status, error) {
	f.finalizeCodeRef = request.ConfirmationCode
	index := f.finalizeCalls
	f.finalizeCalls++
	if index < len(f.finalizeErrors) && f.finalizeErrors[index] != nil {
		return enrollmentflow.Status{}, f.finalizeErrors[index]
	}
	if index < len(f.finalizeStatuses) {
		return f.finalizeStatuses[index], nil
	}
	return enrollmentflow.Status{State: enrollmentapi.StateComplete}, nil
}

func (f *authServiceEnrollmentManager) Cancel(uint64) error {
	f.cancelCalls++
	return nil
}

func TestLoginAgainRefreshesEncryptedSessionBeforeReauthentication(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	client := &authServiceTokenClient{result: protocol.TokenResult{
		State: protocol.AuthResultTokenIssued, AccessToken: "refreshed-access-token", RefreshToken: "refreshed-refresh-token",
	}}
	service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher { return sessionrefresh.New(client, v) }
	authFactoryCalls := 0
	service.newAuthManager = func() (steamCredentialAuthManager, error) {
		authFactoryCalls++
		return &authServiceCredentialManager{steamID: authServiceSteamID}, nil
	}
	registryUpdates := 0
	service.registryUpsertFn = func(got string, state registry.State) error {
		if got != accountID || state != registry.StateActive {
			t.Fatalf("registry update = %q, %q", got, state)
		}
		registryUpdates++
		return nil
	}

	result, err := service.LoginAgain(accountID, grant.Capability)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "refreshed" || !result.RefreshTokenRenewed || !result.CapabilityRefreshRequired || !result.RegistryUpdated {
		t.Fatalf("refresh result = %#v", result)
	}
	if client.calls != 1 || authFactoryCalls != 0 || registryUpdates != 1 {
		t.Fatalf("calls: refresh=%d auth=%d registry=%d", client.calls, authFactoryCalls, registryUpdates)
	}
	account := authServiceAccount(t, service.vault)
	if account.Session.AccessToken != "refreshed-access-token" || account.Session.RefreshToken != "refreshed-refresh-token" {
		t.Fatalf("session was not atomically refreshed: %#v", account.Session)
	}
	if _, err := service.LoginAgain(accountID, grant.Capability); err == nil {
		t.Fatal("pre-refresh generation capability was accepted")
	}
	current := issueSensitiveGrant(t, service, accountID, "request-refresh-reauth1")
	client.result = protocol.TokenResult{}
	client.err = errors.New("remote rejection")
	reauth, err := service.LoginAgain(accountID, current.Capability)
	if err != nil || reauth.State != "reauthentication_required" || reauth.CapabilityRefreshRequired {
		t.Fatalf("reauth result = %#v, err = %v", reauth, err)
	}
	if authFactoryCalls != 0 {
		t.Fatal("refresh failure unexpectedly started credential authentication")
	}
}

// A session that cannot be renewed must reach the UI as a state, never as a
// binding error: an error kills the flow before the credential form opens.
func TestLoginAgainReportsUnrenewableSessionsAsReauthentication(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		result protocol.TokenResult
		err    error
	}{
		{name: "invalid token response", result: protocol.TokenResult{State: protocol.AuthResultTokenIssued}},
		{name: "empty response state", result: protocol.TokenResult{AccessToken: "access-token"}},
		{name: "remote rejection", err: errors.New("remote rejection")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service, accountID, grant := newAuthServiceFixture(t)
			client := &authServiceTokenClient{result: testCase.result, err: testCase.err}
			service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher { return sessionrefresh.New(client, v) }
			service.newAuthManager = func() (steamCredentialAuthManager, error) {
				t.Fatal("refresh failure unexpectedly started credential authentication")
				return nil, nil
			}
			service.registryUpsertFn = func(string, registry.State) error { return nil }

			result, err := service.LoginAgain(accountID, grant.Capability)
			if err != nil {
				t.Fatalf("LoginAgain returned an error instead of a state: %v", err)
			}
			if result.State != "reauthentication_required" || result.CapabilityRefreshRequired {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestCredentialLoginBindsHandleConsumesOnceAndWipesInputs(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)
	grant := issueSensitiveGrant(t, service, accountID, "request-credential-0001")
	manager := &authServiceCredentialManager{steamID: authServiceSteamID}
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return manager, nil }
	service.registryUpsertFn = func(string, registry.State) error { return nil }

	begin, err := service.BeginCredentialLogin(accountID, grant.Capability, "test_account", "password-marker", SteamAuthPurposeLoginAgain)
	if err != nil || begin.Handle == "" || begin.State != string(authflow.StateChallengeRequired) {
		t.Fatalf("begin = %#v, err = %v", begin, err)
	}
	assertZeroed(t, "password", manager.passwordRef)

	foreign := issueSensitiveGrant(t, service, accountID, "request-credential-foreign")
	if _, err := service.SubmitCredentialCode(accountID, foreign.Capability, begin.Handle, "device_code", "A1B2C"); !errors.Is(err, ErrSteamAuthenticationState) {
		t.Fatalf("foreign capability handle error = %v", err)
	}
	if manager.submitCalls != 0 {
		t.Fatal("foreign capability reached authentication manager")
	}

	if _, err := service.SubmitCredentialCode(accountID, grant.Capability, begin.Handle, "device_code", "A1B2C"); err != nil {
		t.Fatal(err)
	}
	assertZeroed(t, "challenge code", manager.codeRef)
	result, err := service.PollCredentialLogin(accountID, grant.Capability, begin.Handle)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "session_updated" || !result.CapabilityRefreshRequired || manager.consumeCalls != 1 {
		t.Fatalf("consume result = %#v, calls = %d", result, manager.consumeCalls)
	}
	account := authServiceAccount(t, service.vault)
	if account.Session.AccessToken != "credential-access-token" || account.Session.RefreshToken != "credential-refresh-token" {
		t.Fatalf("credential session not persisted: %#v", account.Session)
	}
	if _, err := service.PollCredentialLogin(accountID, grant.Capability, begin.Handle); err == nil {
		t.Fatal("consumed handle was replayed")
	}
	if manager.consumeCalls != 1 {
		t.Fatal("authorized credentials were consumed more than once")
	}
	if err := service.revokeLeases(); err != nil {
		t.Fatal(err)
	}
	if !manager.closed {
		t.Fatal("authentication manager was not closed on lease revocation")
	}
}

func TestEnrollmentResumeRevealAckRetryFinalizeAndCancel(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	manager := &authServiceEnrollmentManager{
		resumeStatus: enrollmentflow.Status{
			State: enrollmentapi.StateAwaitingSMS, Confirmation: enrollmentapi.ConfirmationSMS,
			PhoneHint: "***42", Pending: true, Resumed: true, RevocationViewAvailable: true,
		},
		revealCode:     "R12345",
		finalizeErrors: []error{errors.New("temporary finalize failure"), nil, nil},
		finalizeStatuses: []enrollmentflow.Status{
			{},
			{State: enrollmentapi.StateAuthenticatorCodeRetry, Confirmation: enrollmentapi.ConfirmationSMS, Pending: true, RetryAfterSeconds: 5, HasRetryAfter: true},
			{State: enrollmentapi.StateComplete},
		},
	}
	manager.mutateAck = func() error {
		records, err := service.vault.ListRecords()
		if err != nil {
			return err
		}
		raw, err := service.vault.GetRecord(records[0].ID)
		if err != nil {
			return err
		}
		defer wipe(raw)
		_, err = service.vault.PutRecord(accountID, raw)
		return err
	}
	service.enrollmentManager = manager
	service.registryUpsertFn = func(string, registry.State) error { return nil }

	resumed, err := service.ResumeSteamGuardEnrollment(accountID, grant.Capability)
	if err != nil || !resumed.Pending || !resumed.Resumed || resumed.Confirmation != "sms" || !resumed.RegistryUpdated {
		t.Fatalf("resume = %#v, err = %v", resumed, err)
	}
	view, err := service.RevealSteamGuardRevocationCode(accountID, grant.Capability)
	if err != nil || view.Code != "R12345" || view.CapabilityRefreshRequired {
		t.Fatalf("reveal = %#v, err = %v", view, err)
	}
	if _, err := service.RevealSteamGuardRevocationCode(accountID, grant.Capability); !errors.Is(err, ErrRevocationViewAlreadyIssued) {
		t.Fatalf("same capability revealed twice = %v", err)
	}
	foreign := issueSensitiveGrant(t, service, accountID, "request-enrollment-foreign1")
	if _, err := service.AcknowledgeSteamGuardRevocationCode(accountID, foreign.Capability, "R12345"); !errors.Is(err, ErrRevocationAcknowledgment) {
		t.Fatalf("new capability acknowledged reveal = %v", err)
	}
	if _, err := service.AcknowledgeSteamGuardRevocationCode(accountID, grant.Capability, "wrong"); !errors.Is(err, ErrRevocationAcknowledgment) {
		t.Fatalf("wrong acknowledgment error = %v", err)
	}
	acknowledged, err := service.AcknowledgeSteamGuardRevocationCode(accountID, grant.Capability, "R12345")
	if err != nil {
		t.Fatal(err)
	}
	if !acknowledged.CapabilityRefreshRequired {
		t.Fatalf("acknowledgment wrote to the vault but did not signal a capability refresh: %#v", acknowledged)
	}
	assertZeroed(t, "revocation acknowledgment", manager.ackCodeRef)
	if _, err := service.AcknowledgeSteamGuardRevocationCode(accountID, grant.Capability, "R12345"); err == nil {
		t.Fatal("acknowledgment replay succeeded")
	}
	if err := service.EndSensitiveView(grant.Capability, grant.Lease); err != nil {
		t.Fatal(err)
	}
	if _, err := service.FinalizeSteamGuardEnrollment(accountID, grant.Capability, "12345"); err == nil {
		t.Fatal("stale reveal capability finalized enrollment")
	}

	current := issueSensitiveGrant(t, service, accountID, "request-enrollment-current1")
	if _, err := service.FinalizeSteamGuardEnrollment(accountID, current.Capability, "12345"); err == nil {
		t.Fatal("temporary finalization failure was hidden")
	}
	retry, err := service.FinalizeSteamGuardEnrollment(accountID, current.Capability, "12345")
	if err != nil || retry.State != string(enrollmentapi.StateAuthenticatorCodeRetry) || retry.RetryAfterSeconds != 5 || !retry.HasRetryAfter || !retry.CapabilityRefreshRequired {
		t.Fatalf("retry = %#v, err = %v", retry, err)
	}
	assertZeroed(t, "SMS confirmation", manager.finalizeCodeRef)
	current = issueSensitiveGrant(t, service, accountID, "request-enrollment-current2")
	complete, err := service.FinalizeSteamGuardEnrollment(accountID, current.Capability, "54321")
	if err != nil || complete.State != string(enrollmentapi.StateComplete) || !complete.RegistryUpdated {
		t.Fatalf("complete = %#v, err = %v", complete, err)
	}
	if len(service.revocationAcknowledgments) != 0 {
		t.Fatal("completed enrollment retained reveal capability state")
	}

	service.enrollmentManager = manager
	current = issueSensitiveGrant(t, service, accountID, "request-enrollment-cancel1")
	if err := service.CancelSteamGuardEnrollment(accountID, current.Capability); err != nil {
		t.Fatal(err)
	}
	if manager.cancelCalls != 0 {
		t.Fatalf("cancel calls=%d", manager.cancelCalls)
	}
	resumedAfterCancel, err := service.ResumeSteamGuardEnrollment(accountID, current.Capability)
	if err != nil || !resumedAfterCancel.Pending {
		t.Fatalf("pending enrollment was not resumable after cancel: %#v, err=%v", resumedAfterCancel, err)
	}
}

func TestCredentialAuthorizationStartsPendingEnrollmentWithoutExposingTokens(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)
	grant := issueSensitiveGrant(t, service, accountID, "request-enrollment-start1")
	authManager := &authServiceCredentialManager{steamID: authServiceSteamID}
	enrollmentManager := &authServiceEnrollmentManager{startStatus: enrollmentflow.Status{
		State: enrollmentapi.StateAwaitingSMS, Confirmation: enrollmentapi.ConfirmationSMS,
		Pending: true, RevocationViewAvailable: true,
	}}
	service.newAuthManager = func() (steamCredentialAuthManager, error) { return authManager, nil }
	service.enrollmentManager = enrollmentManager
	service.registryUpsertFn = func(string, registry.State) error { return nil }

	begin, err := service.BeginCredentialLogin(accountID, grant.Capability, "test_account", "password-marker", SteamAuthPurposeAddAuthenticator)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.PollCredentialLogin(accountID, grant.Capability, begin.Handle)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "enrollment_pending" || result.Enrollment == nil || !result.Enrollment.Pending ||
		!result.CapabilityRefreshRequired || enrollmentManager.startCalls != 1 || authManager.consumeCalls != 1 {
		t.Fatalf("enrollment start = %#v", result)
	}
	if string(enrollmentManager.startAccessToken) != "credential-access-token" {
		t.Fatal("enrollment manager did not receive the authorized access token")
	}
	raw, err := json.Marshal(result)
	if err != nil || bytes.Contains(raw, enrollmentManager.startAccessToken) {
		t.Fatalf("safe enrollment result exposed bearer credentials: %s", raw)
	}
	if result.Handle != "" {
		t.Fatal("consumed authentication handle was returned")
	}
}

func newAuthServiceFixture(t *testing.T) (*Service, string, SensitiveViewGrant) {
	t.Helper()
	useSettingsRoot(t)
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	if _, err := service.Initialize("correct horse battery staple", ""); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Unlock("correct horse battery staple", vault.ProcessLease); err != nil {
		t.Fatal(err)
	}
	// A ProcessLease pins protected memory until the vault is locked. Left held,
	// leases accumulate across tests until Windows refuses further VirtualLock
	// calls with "insufficient quota" and unrelated tests start failing.
	t.Cleanup(func() { _ = service.vault.Lock() })
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x32}, 20)),
		DeviceID:       "android:00112233-4455-6677-8899-aabbccddeeff",
		AccountName:    "test_account",
		FullyEnrolled:  true,
		Session: &mafile.SessionData{
			SteamID: authServiceSteamID, AccessToken: "old-access-token", RefreshToken: "old-refresh-token",
		},
	}
	raw, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(raw)
	accountID := strconv.FormatUint(authServiceSteamID, 10)
	if _, err := service.vault.PutRecord(accountID, raw); err != nil {
		t.Fatal(err)
	}
	grant := issueSensitiveGrant(t, service, accountID, "request-auth-service1")
	return service, accountID, grant
}

func authServiceAccount(t *testing.T, v *vault.Vault) mafile.Account {
	t.Helper()
	records, err := v.ListRecords()
	if err != nil || len(records) != 1 {
		t.Fatalf("records = %#v, err = %v", records, err)
	}
	account, err := accountFromRecord(v, records[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func assertZeroed(t *testing.T, name string, value []byte) {
	t.Helper()
	for _, current := range value {
		if current != 0 {
			t.Fatalf("%s input was not wiped", name)
		}
	}
}
