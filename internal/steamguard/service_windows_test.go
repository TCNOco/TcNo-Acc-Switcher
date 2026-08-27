//go:build windows

package steamguard

import (
	"errors"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// A vault is only ever created in order to be used straight away: the 2-Factor
// flow calls Initialize and then resumes enrollment, which refuses a locked
// vault.
func TestInitializeLeavesTheVaultUnlockedForEnrollment(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	const password = "separate Steam Guard password"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	if service.vault.IsLocked() {
		t.Fatal("Initialize left the vault locked; enrollment cannot continue")
	}
	// Enrollment authorises the flow first, and that is what rejects a locked vault.
	if _, _, _, err := service.authorizeSteamFlow(qrTestAccountID, "no-such-capability"); errors.Is(err, vault.ErrLocked) {
		t.Fatal("authorizeSteamFlow still reports the vault as locked")
	}
}

// Capabilities are bound to the vault generation they were issued against.
// The 2-Factor modal takes one when it opens, which on a first-time setup is
// before any vault exists, so creating the vault necessarily invalidates it.
// Rejecting it is correct; the modal has to re-acquire before it can continue.
func TestCapabilityIssuedBeforeVaultCreationIsRejectedUntilRefreshed(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	stale := issueSensitiveGrant(t, service, qrTestAccountID, "request-before-vault-0001")

	const password = "separate Steam Guard password"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResumeSteamGuardEnrollment(qrTestAccountID, stale.Capability); !errors.Is(err, capability.ErrInvalidCapability) {
		t.Fatalf("stale capability accepted after vault creation: %v", err)
	}

	refreshed := issueSensitiveGrant(t, service, qrTestAccountID, "request-after-vault-0001")
	if _, err := service.ResumeSteamGuardEnrollment(qrTestAccountID, refreshed.Capability); err != nil {
		t.Fatalf("refreshed capability was rejected: %v", err)
	}
}
