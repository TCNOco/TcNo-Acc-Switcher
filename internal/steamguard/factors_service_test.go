//go:build windows

package steamguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/hwkey"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

func newFactorService(t *testing.T) (*Service, string) {
	t.Helper()
	useSettingsRoot(t)
	service := newServiceForTest()
	const password = "separate Steam Guard password"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(t.TempDir(), "keyfile.txt")
	service.saveKeyfile = func(k vault.Keyfile) (string, error) {
		return saved, os.WriteFile(saved, k.Encode(), 0o600)
	}
	// An unlocked vault holds locked, non-pageable memory. Leaving every test's
	// vault open exhausts the process quota and makes a later test fail with
	// "protected memory is unavailable" for no reason of its own.
	t.Cleanup(func() { _ = service.LockNow() })
	return service, password
}

// Losing the only keyfile with nothing else enrolled makes the vault
// permanently unreadable, so a backup key has to exist first.
func TestKeyfileEnrolmentRequiresABackupKeyFirst(t *testing.T) {
	service, password := newFactorService(t)

	if _, err := service.EnrollVaultKeyfile(password, ""); !errors.Is(err, ErrBackupKeyMissing) {
		t.Fatalf("enrolled a keyfile with no backup key: %v", err)
	}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	if _, err := service.EnrollVaultKeyfile(password, ""); err != nil {
		t.Fatalf("enrolment refused after a backup key existed: %v", err)
	}
}

func TestBackupKeyOpensTheVaultAndReplacesTheOldOne(t *testing.T) {
	service, password := newFactorService(t)

	first, err := service.CreateVaultBackupKey(password)
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	if !status.HasBackupKey || len(status.Factors) != 2 {
		t.Fatalf("after issuing a backup key: %+v", status)
	}

	raw, err := vault.ParseRecoveryCode(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.UnlockWith(vault.Credentials{RecoveryCode: raw}, vault.FixedLease); err != nil {
		t.Fatalf("backup key did not open the vault: %v", err)
	}

	// Issuing a replacement must retire the previous key rather than leaving
	// both valid, or a key the user believes they revoked still opens the vault.
	second, err := service.CreateVaultBackupKey(password)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("reissue returned the same backup key")
	}
	if status, err = service.ListVaultFactors(); err != nil || len(status.Factors) != 2 {
		t.Fatalf("reissue changed the slot count: %+v, %v", status, err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.UnlockWith(vault.Credentials{RecoveryCode: raw}, vault.FixedLease); !errors.Is(err, vault.ErrInvalidPassword) {
		t.Fatalf("the retired backup key still opens the vault: %v", err)
	}
}

// Enrolling never takes away what already worked. Adding a keyfile - with or
// without a password of its own - leaves the password opening the vault exactly
// as it did, because removing a way in is a separate, deliberate act.
func TestEnrollingAKeyfileLeavesThePasswordWorking(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	const keyfilePassword = "a password for the keyfile"
	path, err := service.EnrollVaultKeyfile(password, keyfilePassword)
	if err != nil {
		t.Fatal(err)
	}
	keyfile := readKeyfile(t, path)

	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Unlock(password, vault.FixedLease); err != nil {
		t.Fatalf("the password stopped opening the vault: %v", err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	// The keyfile carries its own password, so the file on its own is not enough.
	if err := service.vault.UnlockWith(vault.Credentials{
		Keyfile: keyfile.Secret,
	}, vault.FixedLease); !errors.Is(err, vault.ErrFactorRequired) {
		t.Fatalf("keyfile alone opened a keyfile-with-password way in: %v", err)
	}
	if err := service.vault.UnlockWith(vault.Credentials{
		Password: keyfilePassword, Keyfile: keyfile.Secret,
	}, vault.FixedLease); err != nil {
		t.Fatalf("keyfile and its password did not open the vault: %v", err)
	}
}

// A keyfile enrolled with no password of its own opens the vault alone, which is
// what makes each way in independent.
func TestKeyfileWithoutAPasswordOpensTheVaultAlone(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	path, err := service.EnrollVaultKeyfile(password, "")
	if err != nil {
		t.Fatal(err)
	}
	keyfile := readKeyfile(t, path)
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.UnlockWith(vault.Credentials{Keyfile: keyfile.Secret}, vault.FixedLease); err != nil {
		t.Fatalf("keyfile alone did not open the vault: %v", err)
	}
}

func readKeyfile(t *testing.T, path string) vault.Keyfile {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	keyfile, err := vault.ParseKeyfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	return keyfile
}

func TestRemoveVaultFactorKeepsAWayIn(t *testing.T) {
	service, password := newFactorService(t)
	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	if status.CanRemoveAFactor {
		t.Fatal("a password-only vault reported a removable factor")
	}
	if err := service.RemoveVaultFactor(password, status.Factors[0].ID); !errors.Is(err, vault.ErrLastSlot) {
		t.Fatalf("removing the only factor: %v", err)
	}
}

// Enrolling a factor is only useful if the vault can then be opened with it.
// Unlock was once password-only, which left a vault with no password-only way in
// unreachable through the app.
func TestUnlockAcceptsEnrolledFactors(t *testing.T) {
	service, password := newFactorService(t)
	code, err := service.CreateVaultBackupKey(password)
	if err != nil {
		t.Fatal(err)
	}
	path, err := service.EnrollVaultKeyfile(password, password)
	if err != nil {
		t.Fatal(err)
	}
	// Removing the password leaves a vault whose only ways in are the keyfile
	// with its password, and the backup key.
	if err := service.RemoveVaultFactor(password, passwordSlotID(t, service)); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.unlockVaultLocked(service.vault, password, false); err == nil {
		t.Fatal("password alone unlocked a vault with no password-only way in")
	}

	// Password plus the keyfile does.
	creds, err := buildVaultCredentials(password, path, "")
	if err != nil {
		t.Fatalf("keyfile could not be read back: %v", err)
	}
	if err := service.unlockVaultWithLocked(service.vault, creds, false); err != nil {
		t.Fatalf("password and keyfile did not unlock: %v", err)
	}

	// So does the backup key on its own.
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	backupCreds, err := buildVaultCredentials("", "", code)
	if err != nil {
		t.Fatalf("backup key was not accepted as a credential: %v", err)
	}
	if err := service.unlockVaultWithLocked(service.vault, backupCreds, false); err != nil {
		t.Fatalf("backup key did not unlock: %v", err)
	}
}

func passwordSlotID(t *testing.T, service *Service) string {
	t.Helper()
	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	for _, factor := range status.Factors {
		if factor.Kind == vault.FactorPassword {
			return factor.ID
		}
	}
	t.Fatalf("no password-only way in: %+v", status)
	return ""
}

// A backup key typed into the password box cannot work, and must not be
// mistaken for a wrong password when supplied as a backup key.
func TestBuildCredentialsRejectsMalformedInput(t *testing.T) {
	if _, err := buildVaultCredentials("pw", "", "not a backup key"); !errors.Is(err, vault.ErrInvalidRecoveryCode) {
		t.Fatalf("malformed backup key error = %v", err)
	}
	if _, err := buildVaultCredentials("pw", "relative/path.txt", ""); !errors.Is(err, vault.ErrInvalidKeyfile) {
		t.Fatalf("relative keyfile path error = %v", err)
	}
}

// The full security-key path with a deterministic fake: enrol, then unlock.
// Everything except the CTAP2 conversation with a real device is covered here.
func TestSecurityKeyEnrolAndUnlock(t *testing.T) {
	service, password := newFactorService(t)
	service.authenticator = &hwkey.Fake{Seed: []byte("test-seed")}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", password); err != nil {
		t.Fatalf("enrolling a security key: %v", err)
	}

	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, factor := range status.Factors {
		for _, kind := range factor.Requires {
			if kind == vault.FactorSecurityKey {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no security-key slot was enrolled: %+v", status)
	}

	// The password alone must not open it, and the key must be asked for and
	// accepted without the caller having to supply anything extra.
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	secret, err := service.evaluateSecurityKey(service.vault)
	if err != nil || len(secret) != hwkey.SecretLength {
		t.Fatalf("evaluate returned %d bytes, err = %v", len(secret), err)
	}
	creds := vault.Credentials{Password: password, SecurityKey: secret}
	if err := service.unlockVaultWithLocked(service.vault, creds, false); err != nil {
		t.Fatalf("password and security key did not unlock: %v", err)
	}
}

// A key that is not the enrolled one must not open the vault, and must not be
// reported as a wrong password either.
func TestSecurityKeyFromAnotherDeviceIsRejected(t *testing.T) {
	service, password := newFactorService(t)
	service.authenticator = &hwkey.Fake{Seed: []byte("the enrolled key")}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", ""); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}

	// A different physical key does not hold the enrolled credential at all, so
	// it produces no secret rather than the wrong one - and that has to read as
	// "wrong key", never as a rejected password.
	service.authenticator = &hwkey.Fake{Seed: []byte("someone else's key")}
	secret, err := service.evaluateSecurityKey(service.vault)
	if !errors.Is(err, hwkey.ErrNoDevice) {
		t.Fatalf("another device's key: err = %v, want %v", err, hwkey.ErrNoDevice)
	}
	if errors.Is(err, vault.ErrInvalidPassword) {
		t.Fatal("an unrecognised security key was reported as a wrong password")
	}
	if len(secret) != 0 {
		t.Fatal("an unrecognised security key still produced key material")
	}
}

// Two keys enrolled, one in the drawer: the one in hand has to work. The driver
// used to ask about each credential in turn and stop at the first that the
// attached key did not hold, which made every key but the first unusable.
func TestASecondSecurityKeyOpensTheVaultOnItsOwn(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	first := &hwkey.Fake{Seed: []byte("the key left at home")}
	second := &hwkey.Fake{Seed: []byte("the key in my pocket")}
	service.authenticator = first
	if err := service.EnrollVaultSecurityKey(password, "Home", ""); err != nil {
		t.Fatal(err)
	}
	service.authenticator = second
	if err := service.EnrollVaultSecurityKey(password, "Pocket", ""); err != nil {
		t.Fatal(err)
	}

	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	if status.SecurityKeyCount != 2 {
		t.Fatalf("security keys enrolled = %d, want 2: %+v", status.SecurityKeyCount, status)
	}

	// Only the second key is attached, and it is not the first credential the
	// vault lists.
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	secret, err := service.evaluateSecurityKey(service.vault)
	if err != nil {
		t.Fatalf("the attached key was not recognised: %v", err)
	}
	if err := service.unlockVaultWithLocked(service.vault, vault.Credentials{SecurityKey: secret}, false); err != nil {
		t.Fatalf("the second enrolled key did not open the vault: %v", err)
	}
}

// Two keys enrolled without names must not become two identical rows with a
// Remove link each.
func TestSecurityKeysGetDistinctNames(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []string{"one", "two"} {
		service.authenticator = &hwkey.Fake{Seed: []byte(seed)}
		if err := service.EnrollVaultSecurityKey(password, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, factor := range status.Factors {
		if factor.Kind != vault.FactorSecurityKey {
			continue
		}
		if seen[factor.Label] {
			t.Fatalf("two security keys share the label %q", factor.Label)
		}
		seen[factor.Label] = true
	}
	if len(seen) != 2 {
		t.Fatalf("labels = %v, want two distinct", seen)
	}

	// And a name can be set afterwards, which is the only way a key enrolled by
	// an older build ever gets one.
	var target string
	for _, factor := range status.Factors {
		if factor.Kind == vault.FactorSecurityKey {
			target = factor.ID
			break
		}
	}
	if err := service.RenameVaultFactor(password, target, "Desk drawer"); err != nil {
		t.Fatalf("renaming a security key: %v", err)
	}
	if status, err = service.ListVaultFactors(); err != nil {
		t.Fatal(err)
	}
	var renamed bool
	for _, factor := range status.Factors {
		if factor.ID == target && factor.Label == "Desk drawer" {
			renamed = true
		}
	}
	if !renamed {
		t.Fatalf("rename did not take: %+v", status.Factors)
	}
}

// No key attached is not a wrong password, and enrolling one still requires a
// backup key first.
func TestSecurityKeyEnrolmentGuards(t *testing.T) {
	service, password := newFactorService(t)
	service.authenticator = &hwkey.Fake{Seed: []byte("seed")}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", ""); !errors.Is(err, ErrBackupKeyMissing) {
		t.Fatalf("enrolled a security key with no backup key: %v", err)
	}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	service.authenticator = &hwkey.Fake{Absent: true}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", ""); !errors.Is(err, hwkey.ErrNoDevice) {
		t.Fatalf("enrolment with no key attached: %v", err)
	}
}

// Once a slot needs a password and a keyfile, the password alone can no longer
// derive the vault key. Factor management used to re-derive from the password,
// so enrolling anything else became impossible the moment the first combined
// factor was added. Management now works from the unlocked vault instead.
func TestFactorsRemainManageableAfterCombinedEnrolment(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	if _, err := service.EnrollVaultKeyfile(password, password); err != nil {
		t.Fatal(err)
	}

	// The vault is unlocked from the enrolment above, which is the state the
	// settings screen is in. Every management action must still work.
	service.authenticator = &hwkey.Fake{Seed: []byte("a key")}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", ""); err != nil {
		t.Fatalf("enrolling a security key after a combined slot: %v", err)
	}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatalf("replacing the backup key after a combined slot: %v", err)
	}
	// Only one keyfile is allowed, so that "Remove keyfile" cannot leave a second
	// file on disk still opening the vault.
	if _, err := service.EnrollVaultKeyfile(password, ""); !errors.Is(err, ErrKeyfileAlreadyEnrolled) {
		t.Fatalf("enrolled a second keyfile: %v", err)
	}

	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	if !status.CanRemoveAFactor {
		t.Fatalf("expected several ways in: %+v", status)
	}
	var removable string
	for _, factor := range status.Factors {
		if factor.Kind == vault.FactorSecurityKey && factor.Removable {
			removable = factor.ID
		}
	}
	if removable == "" {
		t.Fatalf("no removable security key: %+v", status)
	}
	if err := service.RemoveVaultFactor(password, removable); err != nil {
		t.Fatalf("removing a factor after a combined slot: %v", err)
	}
}

// The removability rules, which is what the settings screen greys out on.
func TestRemovabilityKeepsAUsableWayIn(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	// Password plus a backup key. The password is the only way in the user can
	// actually open the app with, so it stays; the backup key may go, since
	// nothing losable is enrolled against it yet.
	for _, factor := range status.Factors {
		switch factor.Kind {
		case vault.FactorPassword:
			if factor.Removable || factor.Blocks != blockLastInteractive {
				t.Fatalf("the only usable way in was removable: %+v", factor)
			}
		case vault.FactorRecoveryCode:
			if !factor.Removable {
				t.Fatalf("backup key blocked with nothing losable enrolled: %+v", factor)
			}
		}
	}

	keyfilePath, err := service.EnrollVaultKeyfile(password, "")
	if err != nil {
		t.Fatal(err)
	}
	if status, err = service.ListVaultFactors(); err != nil {
		t.Fatal(err)
	}
	for _, factor := range status.Factors {
		switch factor.Kind {
		case vault.FactorRecoveryCode:
			// A keyfile can be lost, so the backup key stays put.
			if factor.Removable || factor.Blocks != blockBackupNeeded {
				t.Fatalf("backup key removable alongside a keyfile: %+v", factor)
			}
		default:
			if !factor.Removable {
				t.Fatalf("%s should be removable once two usable ways in exist: %+v", factor.Kind, factor)
			}
		}
	}

	// Removing the password leaves the keyfile, which then cannot go either.
	if err := service.RemoveVaultFactor(password, passwordSlotID(t, service)); err != nil {
		t.Fatal(err)
	}
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockVaultForManagement("", keyfilePath, ""); err != nil {
		t.Fatal(err)
	}
	if status, err = service.ListVaultFactors(); err != nil {
		t.Fatal(err)
	}
	for _, factor := range status.Factors {
		if factor.Removable {
			t.Fatalf("%s was removable when it was the last usable way in: %+v", factor.Kind, factor)
		}
	}
}

// Changing the password must not report success while some way in still answers
// to the old one.
func TestChangePasswordRefusesToLeaveTheOldOneWorking(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	keyfilePath, err := service.EnrollVaultKeyfile(password, password)
	if err != nil {
		t.Fatal(err)
	}

	const next = "a replacement Steam Guard password"
	// Without the keyfile the keyfile slot cannot be re-derived, and it holds the
	// same password - so changing only the password-only slot would leave the old
	// password opening the vault for anyone holding that file.
	err = service.ChangePasswordWithFactors(password, next, "", "", "")
	if !errors.Is(err, vault.ErrPasswordStillInUse) {
		t.Fatalf("change password without the keyfile: err = %v, want %v", err, vault.ErrPasswordStillInUse)
	}
	// And nothing was written: the old password still works, the new one does not.
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockVaultForManagement(next, "", ""); err == nil {
		t.Fatal("the refused password change was committed anyway")
	}
	if err := service.UnlockVaultForManagement(password, "", ""); err != nil {
		t.Fatalf("the refused change disturbed the old password: %v", err)
	}

	if err := service.ChangePasswordWithFactors(password, next, "", keyfilePath, ""); err != nil {
		t.Fatalf("change password with the keyfile: %v", err)
	}
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockVaultForManagement(password, keyfilePath, ""); err == nil {
		t.Fatal("the old password still opens the vault after the change")
	}
	if err := service.UnlockVaultForManagement(next, keyfilePath, ""); err != nil {
		t.Fatalf("the new password does not open the vault: %v", err)
	}
}

// The test above only covered a vault still unlocked from enrolment. The unlock
// lease is five minutes, so in practice the settings screen is reached with the
// vault locked - and a password alone cannot reopen a password-and-keyfile slot,
// which made every factor action fail with "requires an enrolled factor".
func TestManagementReopensALockedCombinedVault(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	keyfilePath, err := service.EnrollVaultKeyfile(password, password)
	if err != nil {
		t.Fatal(err)
	}
	// Enrolling leaves the password working, so the password-only way in is
	// removed here to get the shape this test is about: a vault the password
	// alone cannot open.
	if err := service.RemoveVaultFactor(password, passwordSlotID(t, service)); err != nil {
		t.Fatal(err)
	}
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}

	if err := service.UnlockVaultForManagement(password, "", ""); !errors.Is(err, vault.ErrFactorRequired) {
		t.Fatalf("password alone unlocked a combined vault: %v", err)
	}
	if err := service.UnlockVaultForManagement(password, keyfilePath, ""); err != nil {
		t.Fatalf("password and keyfile did not unlock for management: %v", err)
	}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatalf("management action after unlocking: %v", err)
	}

	// Changing the password rebuilds the slot, so it needs the keyfile too and
	// must leave the pair still able to open the vault afterwards.
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	const next = "a different Steam Guard password"
	if err := service.ChangePasswordWithFactors(password, next, "", "", ""); !errors.Is(err, vault.ErrFactorRequired) {
		t.Fatalf("changed the password without the keyfile: %v", err)
	}
	if err := service.ChangePasswordWithFactors(password, next, "", keyfilePath, ""); err != nil {
		t.Fatalf("changing the password with the keyfile: %v", err)
	}
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockVaultForManagement(next, keyfilePath, ""); err != nil {
		t.Fatalf("new password and the same keyfile did not unlock: %v", err)
	}
}

// The promise the settings screen makes: enrolled without a password of their
// own, the password, the keyfile and each security key are alternatives, and any
// one of them opens the vault by itself.
func TestEveryEnrolledFactorOpensTheVaultOnItsOwn(t *testing.T) {
	service, password := newFactorService(t)
	service.authenticator = &hwkey.Fake{Seed: []byte("the only key")}
	code, err := service.CreateVaultBackupKey(password)
	if err != nil {
		t.Fatal(err)
	}
	keyfilePath, err := service.EnrollVaultKeyfile(password, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", ""); err != nil {
		t.Fatal(err)
	}

	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	if !status.PasswordOpens || !status.HasKeyfile || status.SecurityKeyCount != 1 || !status.HasBackupKey {
		t.Fatalf("factor summary does not describe four ways in: %+v", status)
	}
	for _, factor := range status.Factors {
		if factor.RequiresPassword {
			t.Fatalf("%q was enrolled needing the password as well: %+v", factor.Label, factor)
		}
	}
	if got := status.Factors[len(status.Factors)-1].Label; got != "YubiKey 5C" {
		t.Fatalf("security key label = %q, want the name the user chose", got)
	}

	keyfile := readKeyfile(t, keyfilePath)
	backupKey, err := vault.ParseRecoveryCode(code)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := service.evaluateSecurityKey(service.vault)
	if err != nil {
		t.Fatal(err)
	}
	for name, creds := range map[string]vault.Credentials{
		"password":     {Password: password},
		"keyfile":      {Keyfile: keyfile.Secret},
		"security key": {SecurityKey: secret},
		"backup key":   {RecoveryCode: backupKey},
	} {
		t.Run(name, func(t *testing.T) {
			if err := service.vault.Lock(); err != nil {
				t.Fatal(err)
			}
			if err := service.vault.UnlockWith(creds, vault.FixedLease); err != nil {
				t.Fatalf("%s did not open the vault on its own: %v", name, err)
			}
		})
	}
}

// A vault can legitimately have no password - the user removed it in favour of a
// security key - so there has to be a way back to one.
func TestPasswordCanBeRemovedAndAddedBack(t *testing.T) {
	service, password := newFactorService(t)
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	keyfilePath, err := service.EnrollVaultKeyfile(password, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RemoveVaultFactor(password, passwordSlotID(t, service)); err != nil {
		t.Fatal(err)
	}
	status, err := service.ListVaultFactors()
	if err != nil {
		t.Fatal(err)
	}
	if status.PasswordOpens {
		t.Fatalf("the password still opens the vault after removal: %+v", status)
	}

	// Adding one back needs an existing way in to authenticate with, which is the
	// keyfile - the password is exactly what is missing.
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockVaultForManagement("", keyfilePath, ""); err != nil {
		t.Fatalf("keyfile did not open the vault for management: %v", err)
	}
	const next = "a brand new Steam Guard password"
	if err := service.EnrollVaultPassword("", next); err != nil {
		t.Fatalf("adding a password back: %v", err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Unlock(next, vault.FixedLease); err != nil {
		t.Fatalf("the new password did not open the vault: %v", err)
	}
	// A second one would be a duplicate way in that the settings list could not
	// tell apart, and the button that offers it is hidden once one exists.
	if err := service.EnrollVaultPassword("", "yet another password"); !errors.Is(err, ErrPasswordAlreadyEnrolled) {
		t.Fatalf("enrolled a second password-only way in: %v", err)
	}
}

// The settings screen asks the user to prove themselves before changing who can
// open the vault. The vault is usually already open from the previous action, so
// unlocking proves nothing and the answer has to be checked on its own.
func TestManagementAuthenticationIsCheckedOnAnOpenVault(t *testing.T) {
	service, password := newFactorService(t)
	if err := service.UnlockVaultForManagement(password, "", ""); err != nil {
		t.Fatal(err)
	}
	if service.vault.IsLocked() {
		t.Fatal("expected the vault to be open for this case")
	}
	if err := service.UnlockVaultForManagement("not the password", "", ""); err == nil {
		t.Fatal("a wrong password was accepted while the vault happened to be open")
	}
	// And the right one still is, so the check is not simply refusing everything.
	if err := service.UnlockVaultForManagement(password, "", ""); err != nil {
		t.Fatalf("the correct password was refused on an open vault: %v", err)
	}
}

// A way in pairing the password with a security key can only be re-keyed with
// the device. The unlock happens on the standalone password slot and never
// touches the key, so the change would be refused for a factor the user is
// holding but was never asked for.
func TestChangePasswordAsksTheSecurityKeyItNeeds(t *testing.T) {
	service, password := newFactorService(t)
	service.authenticator = &hwkey.Fake{Seed: []byte("the key on my keyring")}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", password); err != nil {
		t.Fatal(err)
	}

	const next = "a replacement Steam Guard password"
	if err := service.ChangePasswordWithFactors(password, next, "", "", ""); err != nil {
		t.Fatalf("changing the password with a key-and-password way in: %v", err)
	}

	// Both ways in moved to the new password, and neither answers to the old one.
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlockVaultForManagement(password, "", ""); err == nil {
		t.Fatal("the old password still opens the vault")
	}
	if err := service.UnlockVaultForManagement(next, "", ""); err != nil {
		t.Fatalf("the new password does not open the vault: %v", err)
	}
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	secret, err := service.evaluateSecurityKey(service.vault)
	if err != nil {
		t.Fatal(err)
	}
	stale := vault.Credentials{Password: password, SecurityKey: secret}
	if err := service.unlockVaultWithLocked(service.vault, stale, false); err == nil {
		t.Fatal("the old password still opens the vault with the security key")
	}
	fresh := vault.Credentials{Password: next, SecurityKey: secret}
	if err := service.unlockVaultWithLocked(service.vault, fresh, false); err != nil {
		t.Fatalf("the new password and the key do not open the vault: %v", err)
	}
}

// The unlock screen asks what the vault accepts before anyone has opened it, so
// this has to answer from the header while the vault is locked. Without it the
// screen cannot know to offer the security key, and a vault whose only way in is
// a key cannot be opened from there at all.
func TestSettingsStatusReportsWaysInWhileLocked(t *testing.T) {
	service, password := newFactorService(t)
	service.authenticator = &hwkey.Fake{Seed: []byte("a key")}
	if _, err := service.CreateVaultBackupKey(password); err != nil {
		t.Fatal(err)
	}
	if err := service.EnrollVaultSecurityKey(password, "YubiKey 5C", ""); err != nil {
		t.Fatal(err)
	}
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}

	status, err := service.GetSettingsStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Unlocked {
		t.Fatal("expected the vault to be locked for this case")
	}
	if !status.HasSecurityKey {
		t.Fatalf("a locked vault did not report its enrolled security key: %+v", status)
	}
	if !status.PasswordOpens {
		t.Fatalf("a locked vault did not report that the password opens it: %+v", status)
	}

	// And a vault with no key says so, or the screen would offer a device that
	// is never going to be asked for.
	plain, plainPassword := newFactorService(t)
	if err := plain.LockNow(); err != nil {
		t.Fatal(err)
	}
	_ = plainPassword
	plainStatus, err := plain.GetSettingsStatus()
	if err != nil {
		t.Fatal(err)
	}
	if plainStatus.HasSecurityKey {
		t.Fatalf("a password-only vault reported a security key: %+v", plainStatus)
	}
}
