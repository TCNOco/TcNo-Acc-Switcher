package steamguard

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/battlenetauth"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steambrowser"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

type battleNetVerificationClient struct {
	account  battlenetauth.Account
	err      error
	calls    int
	response []byte
}

func (c *battleNetVerificationClient) EnrollWithRecovery(_ context.Context, _ string, preserve func([]byte) error, ready ...func(battlenetauth.EnrollmentIdentity) error) (battlenetauth.Account, error) {
	c.calls++
	for _, notify := range ready {
		if err := notify(battlenetauth.EnrollmentIdentity{BattleTag: "Fixture#1234"}); err != nil {
			return battlenetauth.Account{}, err
		}
	}
	response := c.response
	if response == nil && c.err == nil {
		response, _ = json.Marshal(c.account)
	}
	if response != nil && preserve != nil {
		if err := preserve(response); err != nil {
			return battlenetauth.Account{}, err
		}
	}
	return c.account, c.err
}

func battleNetVerificationFixture(t *testing.T, client *battleNetVerificationClient, options ...vault.Option) (*Service, *battleNetOperation, SensitiveViewGrant) {
	t.Helper()
	root, err := os.MkdirTemp("", "battlenet-verification-")
	if err != nil {
		t.Fatal(err)
	}
	paths.ResetForTest(root)
	s := newServiceForTest(options...)
	s.setMainContentProtectionFn = func(bool) error { return nil }
	if _, err := s.Initialize("correct horse battery staple", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.vault.Unlock("correct horse battery staple", vault.ProcessLease); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.vault.Lock() })
	var grant SensitiveViewGrant
	s.emitMainWindowEventFn = func(name string, data any) error {
		if name == SensitiveViewGrantEvent {
			grant = data.(SensitiveViewGrant)
		}
		return nil
	}
	if err := s.RequestBattleNetView("verification-request-fixture"); err != nil {
		t.Fatal(err)
	}
	op := &battleNetOperation{vault: s.vault, status: BattleNetEnrollmentStatus{Handle: "fixture", State: "waiting"}, token: grant.Capability, expires: time.Now().Add(time.Minute)}
	s.battleNet = &battleNetRuntime{operation: op, client: client}
	t.Cleanup(func() { _ = s.EndSensitiveView(grant.Capability, grant.Lease) })
	return s, op, grant
}

func TestBattleNetVerificationNoticeDoesNotDiscardSavedAuthenticator(t *testing.T) {
	client := &battleNetVerificationClient{account: battlenetauth.Account{
		Version: 1, Serial: "EU-1234-5678-9012", DeviceSecret: "3132333435363738393031323334353637383930",
		RestoreCode: "0123456789", Pending: true, RequiresAccountSetup: true,
	}}
	s, op, _ := battleNetVerificationFixture(t, client)
	closed := false
	s.battleNet.readLoginIdentity = func(_ context.Context, _ *steambrowser.Service, profile string) (steambrowser.BattleNetAccountIdentity, error) {
		if profile != "battlenet-fixture" {
			t.Fatal("wrong login profile")
		}
		if !s.mu.TryLock() {
			t.Fatal("native identity read holds the service lock")
		}
		s.mu.Unlock()
		return steambrowser.BattleNetAccountIdentity{Email: "fixture@example.test"}, nil
	}
	s.battleNet.closeEnrollmentBrowser = func(_ *steambrowser.Service, profile string) {
		a, err := battleNetRecord(s.vault, client.account.Serial)
		if err != nil || a.AccountName != "Fixture#1234" || a.AccountLink != "fixture@example.test" || profile != "battlenet-fixture" || op.status.State != "pending-backup" {
			t.Fatal("browser closed before named authenticator was saved")
		}
		closed = true
	}
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if !closed {
		t.Fatal("successful enrollment browser was left open")
	}
	if op.status.State != "pending-backup" || op.status.Error != "" {
		t.Fatal("valid authenticator did not reach recovery backup")
	}
	a, err := battleNetRecord(s.vault, client.account.Serial)
	if err != nil || !a.RequiresAccountSetup || !a.Pending || a.DeviceSecret != client.account.DeviceSecret || a.BrowserProfile != "battlenet-fixture" {
		t.Fatal("saved authenticator lost credentials, account setup notice, or browser profile")
	}
	if !battleNetSummary(a).RequiresAccountSetup {
		t.Fatal("account setup notice was not exposed in the safe summary")
	}
	if s.battleNet.pending != nil {
		t.Fatal("committed authenticator retained a pending memory copy")
	}
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if client.calls != 1 {
		t.Fatal("account setup notice caused repeated enrollment")
	}
}

func TestBattleNetVerificationWithoutCredentialsOffersBoundAccountPage(t *testing.T) {
	client := &battleNetVerificationClient{err: battlenetauth.ErrAccountVerification}
	s, op, grant := battleNetVerificationFixture(t, client)
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if op.status.State != "verification-required" || client.calls != 1 {
		t.Fatal("missing account setup step was reported as generic failure")
	}
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if client.calls != 1 {
		t.Fatal("verification request retried enrollment")
	}
	for _, pair := range [][2]string{{"another-operation", grant.Capability}, {op.status.Handle, "wrong-capability"}} {
		if err := s.OpenBattleNetEnrollmentAccount(pair[0], pair[1]); !errors.Is(err, ErrSensitiveView) {
			t.Fatal("unbound account page access was accepted")
		}
	}
	if err := s.OpenBattleNetEnrollmentAccount(op.status.Handle, grant.Capability); !errors.Is(err, steambrowser.ErrUnsupportedPlatform) {
		t.Fatal("valid binding did not reach the browser handoff")
	}
	if err := s.EndSensitiveView(grant.Capability, grant.Lease); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenBattleNetEnrollmentAccount(op.status.Handle, grant.Capability); err == nil || errors.Is(err, steambrowser.ErrUnsupportedPlatform) {
		t.Fatal("revoked view reached the browser")
	}
}

func TestBattleNetVerificationCanBeCanceled(t *testing.T) {
	s, op, grant := battleNetVerificationFixture(t, &battleNetVerificationClient{err: battlenetauth.ErrAccountVerification})
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if err := s.CancelBattleNetEnrollment(op.status.Handle, grant.Capability); err != nil || op.status.State != "canceled" {
		t.Fatal("verification step could not be canceled")
	}
	if err := s.OpenBattleNetEnrollmentAccount(op.status.Handle, grant.Capability); !errors.Is(err, ErrSensitiveView) {
		t.Fatal("canceled setup could open an account page")
	}
}

func TestBattleNetResponseRecoverySurvivesLossOfPendingMemory(t *testing.T) {
	client := &battleNetVerificationClient{err: battlenetauth.ErrInvalid, response: []byte(`{"serial":"EU123456789012","restoreCode":"0123456789","deviceSecret":"3132333435363738393031323334353637383930"}`)}
	s, op, grant := battleNetVerificationFixture(t, client)
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if op.status.State != "recovery-required" || s.battleNet.pending == nil {
		t.Fatal("response validation failure did not enter protected recovery")
	}
	if err := s.battleNet.pending.Destroy(); err != nil {
		t.Fatal(err)
	}
	s.battleNet.pending = nil
	s.battleNet.operation = nil
	if _, err := s.BeginBattleNetEnrollment(grant.Capability); !errors.Is(err, ErrBattleNetResponseRecovery) {
		t.Fatal("new enrollment was allowed with an unresolved disk receipt")
	}
	if err := s.RecoverBattleNetEnrollments(grant.Capability); err != nil {
		t.Fatal(err)
	}
	a, err := battleNetRecord(s.vault, "EU-1234-5678-9012")
	if err != nil || a.DeviceSecret != "3132333435363738393031323334353637383930" || !a.Pending || a.AccountName != "Fixture#1234" {
		t.Fatal("encrypted response was not recovered")
	}
	payloads, err := s.vault.BattleNetRecoveryPayloads()
	if err != nil || len(payloads) != 0 {
		t.Fatal("resolved recovery receipt remained pending")
	}
	if client.calls != 1 {
		t.Fatal("recovery called the enrollment API again")
	}
}

func TestBattleNetMalformedResponseStaysRecoverableAndBlocksEnrollment(t *testing.T) {
	client := &battleNetVerificationClient{err: battlenetauth.ErrInvalid, response: []byte(`{"deviceSecret":"unsupported-shape"}`)}
	s, op, grant := battleNetVerificationFixture(t, client)
	s.battleNet.closeEnrollmentBrowser = func(*steambrowser.Service, string) { t.Fatal("failed enrollment browser closed") }
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if op.status.State != "recovery-required" {
		t.Fatal("malformed response allowed normal setup retry")
	}
	if err := s.RecoverBattleNetEnrollments(grant.Capability); !errors.Is(err, ErrBattleNetResponseRecovery) {
		t.Fatal("malformed response was accepted")
	}
	if s.battleNet.pending == nil {
		t.Fatal("failed recovery destroyed the protected response")
	}
	if _, err := s.BeginBattleNetEnrollment(grant.Capability); err == nil {
		t.Fatal("failed recovery allowed another enrollment")
	}
	payloads, err := s.vault.BattleNetRecoveryPayloads()
	if err != nil || len(payloads) != 1 {
		t.Fatal("failed recovery lost the encrypted receipt")
	}
	for _, payload := range payloads {
		wipe(payload.Plaintext)
	}
	if client.calls != 1 {
		t.Fatal("failed recovery retried enrollment")
	}
	t.Cleanup(func() {
		if s.battleNet.pending != nil {
			_ = s.battleNet.pending.Destroy()
			s.battleNet.pending = nil
		}
	})
}

type battleNetRecoveryHardener struct{ fail bool }

func (h *battleNetRecoveryHardener) HardenDir(string) error {
	if h.fail {
		return errors.New("synthetic storage failure")
	}
	return nil
}
func (h *battleNetRecoveryHardener) HardenFile(string) error { return h.HardenDir("") }

func TestBattleNetReceiptWriteFailureRetainsProtectedResponse(t *testing.T) {
	hardener := &battleNetRecoveryHardener{}
	client := &battleNetVerificationClient{account: battlenetauth.Account{Version: 1, Serial: "EU-1234-5678-9012", DeviceSecret: "3132333435363738393031323334353637383930", Pending: true}}
	s, op, grant := battleNetVerificationFixture(t, client, vault.WithHardener(hardener))
	hardener.fail = true
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if op.status.State != "recovery-required" || s.battleNet.pending == nil || client.calls != 1 {
		t.Fatal("storage failure did not retain the completed response")
	}
	hardener.fail = false
	if err := s.RecoverBattleNetEnrollments(grant.Capability); err != nil {
		t.Fatal(err)
	}
	if _, err := battleNetRecord(s.vault, "EU-1234-5678-9012"); err != nil {
		t.Fatal("protected response could not be recovered after storage became available")
	}
	if s.battleNet.pending != nil || client.calls != 1 {
		t.Fatal("recovery retained memory or repeated enrollment")
	}
}

func TestBattleNetSetupOnlyReceiptDoesNotBlockVerificationRetry(t *testing.T) {
	client := &battleNetVerificationClient{err: battlenetauth.ErrAccountVerification, response: []byte(`{"requireHealup":true}`)}
	s, op, _ := battleNetVerificationFixture(t, client)
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
	if op.status.State != "verification-required" || s.battleNet.pending != nil {
		t.Fatal("setup-only response was left in credential recovery")
	}
	pending, err := battleNetRecoveryPending(s.vault)
	if err != nil || pending {
		t.Fatal("setup-only receipt blocked verification flow")
	}
}
