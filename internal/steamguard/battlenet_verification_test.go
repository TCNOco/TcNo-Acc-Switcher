package steamguard

import (
	"context"
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
	account battlenetauth.Account
	err     error
	calls   int
}

func (c *battleNetVerificationClient) Enroll(context.Context, string) (battlenetauth.Account, error) {
	c.calls++
	return c.account, c.err
}

func battleNetVerificationFixture(t *testing.T, client *battleNetVerificationClient) (*Service, *battleNetOperation, SensitiveViewGrant) {
	t.Helper()
	root, err := os.MkdirTemp("", "battlenet-verification-")
	if err != nil {
		t.Fatal(err)
	}
	paths.ResetForTest(root)
	s := newServiceForTest()
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
	s.completeBattleNetEnrollment(op, "EU-012345678901234567890123456789")
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
