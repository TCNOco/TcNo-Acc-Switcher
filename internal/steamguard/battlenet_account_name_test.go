package steamguard

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/battlenetauth"
	"TcNo-Acc-Switcher/internal/steambrowser"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

func battleNetNameGrant(t *testing.T, s *Service) SensitiveViewGrant {
	t.Helper()
	var grant SensitiveViewGrant
	s.emitMainWindowEventFn = func(name string, data any) error {
		if name == SensitiveViewGrantEvent {
			grant = data.(SensitiveViewGrant)
		}
		return nil
	}
	if err := s.RequestBattleNetView("account-name-fixture"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.EndSensitiveView(grant.Capability, grant.Lease) })
	return grant
}

func TestBattleNetAccountNameStaysBoundToRecordAndPreservesRename(t *testing.T) {
	s, _, _ := battleNetVerificationFixture(t, &battleNetVerificationClient{})
	a := battlenetauth.Account{Version: 1, Serial: "EU-1234-5678-9012", DeviceSecret: "3132333435363738393031323334353637383930", AccountName: "test@example.invalid", BrowserProfile: "battlenet-name-fixture"}
	raw, _ := json.Marshal(a)
	if _, err := s.vault.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw); err != nil {
		t.Fatal(err)
	}
	grant := battleNetNameGrant(t, s)
	reads := 0
	s.battleNet.readAccountName = func(_ context.Context, _ *steambrowser.Service, profile string) (steambrowser.BattleNetAccountIdentity, error) {
		reads++
		if profile != a.BrowserProfile {
			t.Fatal("another authenticator profile was read")
		}
		return steambrowser.BattleNetAccountIdentity{Name: "Example#1234"}, nil
	}
	if changed, err := s.RefreshBattleNetAccountName(a.Serial, "wrong-token"); err == nil || changed || reads != 0 {
		t.Fatal("unauthorized name lookup reached the account browser")
	}
	if changed, err := s.RefreshBattleNetAccountName(a.Serial, grant.Capability); err != nil || !changed {
		t.Fatal("signed-in name was not saved")
	}
	stored, err := battleNetRecord(s.vault, a.Serial)
	if err != nil || stored.AccountName != "Example#1234" || stored.DeviceSecret != a.DeviceSecret || stored.AccountLink != "" || battleNetSummary(stored).Label != "Example#1234" {
		t.Fatal("account name update changed credentials or account linking")
	}
	grant = battleNetNameGrant(t, s)
	generation := s.vault.Generation()
	if changed, err := s.RefreshBattleNetAccountName(a.Serial, grant.Capability); err != nil || changed || s.vault.Generation() != generation {
		t.Fatal("unchanged name rotated vault capabilities")
	}
	if err := s.SetBattleNetLabel(a.Serial, "My main account", grant.Capability); err != nil {
		t.Fatal(err)
	}
	grant = battleNetNameGrant(t, s)
	before := reads
	if changed, err := s.RefreshBattleNetAccountName(a.Serial, grant.Capability); err != nil || changed || reads != before+1 {
		t.Fatal("unchanged automatic metadata replaced a manual rename")
	}
}

func TestBattleNetAccountNameRechecksAuthorizationAfterBrowserRead(t *testing.T) {
	s, _, _ := battleNetVerificationFixture(t, &battleNetVerificationClient{})
	a := battlenetauth.Account{Version: 1, Serial: "EU-1234-5678-9012", DeviceSecret: "3132333435363738393031323334353637383930"}
	raw, _ := json.Marshal(a)
	if _, err := s.vault.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw); err != nil {
		t.Fatal(err)
	}
	grant := battleNetNameGrant(t, s)
	s.battleNet.readAccountName = func(_ context.Context, _ *steambrowser.Service, profile string) (steambrowser.BattleNetAccountIdentity, error) {
		if profile != battleNetBrowserProfile(a) {
			t.Fatal("restored authenticator profile did not match Account security")
		}
		if err := s.SetBattleNetLabel(a.Serial, "Chosen name", grant.Capability); err != nil {
			t.Fatal(err)
		}
		return steambrowser.BattleNetAccountIdentity{Name: "Late#1234"}, nil
	}
	if changed, err := s.RefreshBattleNetAccountName(a.Serial, grant.Capability); err == nil || changed {
		t.Fatal("stale browser result retained write authorization")
	}
	stored, err := battleNetRecord(s.vault, a.Serial)
	if err != nil || stored.Label != "Chosen name" || stored.AccountName != "" {
		t.Fatal("stale browser result replaced a newer manual rename")
	}
}

func TestBattleNetAccountIdentityLinksEmailWithoutReplacingManualName(t *testing.T) {
	s, _, _ := battleNetVerificationFixture(t, &battleNetVerificationClient{})
	a := battlenetauth.Account{Version: 1, Serial: "EU-1234-5678-9012", DeviceSecret: "3132333435363738393031323334353637383930", AccountName: "Example#1234", Label: "My account"}
	raw, _ := json.Marshal(a)
	if _, err := s.vault.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw); err != nil {
		t.Fatal(err)
	}
	s.battleNet.readAccountName = func(context.Context, *steambrowser.Service, string) (steambrowser.BattleNetAccountIdentity, error) {
		return steambrowser.BattleNetAccountIdentity{Name: "Example#1234", Email: "test@example.invalid"}, nil
	}
	grant := battleNetNameGrant(t, s)
	if changed, err := s.RefreshBattleNetAccountName(a.Serial, grant.Capability); err != nil || !changed {
		t.Fatal("unchanged name prevented email association")
	}
	stored, err := battleNetRecord(s.vault, a.Serial)
	if err != nil || stored.Label != a.Label || stored.AccountLink != "test@example.invalid" || stored.DeviceSecret != a.DeviceSecret {
		t.Fatal("account association replaced a name or credential")
	}
	grant = battleNetNameGrant(t, s)
	if err := s.LinkBattleNetAccount(a.Serial, "chosen@example.invalid", "", grant.Capability); err != nil {
		t.Fatal(err)
	}
	grant = battleNetNameGrant(t, s)
	if changed, err := s.RefreshBattleNetAccountName(a.Serial, grant.Capability); err != nil || changed {
		t.Fatal("automatic association overwrote an explicit account link")
	}
}

type battleNetNumericClipboard struct{ copied int }

func (*battleNetNumericClipboard) Copy(string, time.Duration) error { return nil }
func (*battleNetNumericClipboard) Clear() (bool, error)             { return false, nil }
func (*battleNetNumericClipboard) Close() error                     { return nil }
func (c *battleNetNumericClipboard) CopyNumeric(code string, _ time.Duration) error {
	if len(code) != 8 {
		return battlenetauth.ErrInvalid
	}
	c.copied++
	return nil
}

func TestBattleNetContextCopyRechecksAccountAssociation(t *testing.T) {
	s, _, _ := battleNetVerificationFixture(t, &battleNetVerificationClient{})
	a := battlenetauth.Account{Version: 1, Serial: "EU-1234-5678-9012", DeviceSecret: "3132333435363738393031323334353637383930", AccountLink: "test@example.invalid"}
	raw, _ := json.Marshal(a)
	if _, err := s.vault.PutProviderRecord(vault.ProviderBattleNet, a.Serial, raw); err != nil {
		t.Fatal(err)
	}
	grant := battleNetNameGrant(t, s)
	clipboard := &battleNetNumericClipboard{}
	s.clipboard = clipboard
	if err := s.CopyBattleNetCodeForAccount(a.Serial, "wrong@example.invalid", grant.Capability); err == nil || clipboard.copied != 0 {
		t.Fatal("context menu copied another account's authenticator")
	}
	if err := s.CopyBattleNetCodeForAccount(a.Serial, " TEST@example.invalid ", grant.Capability); err != nil || clipboard.copied != 1 {
		t.Fatal("matching account could not copy its code")
	}
}
