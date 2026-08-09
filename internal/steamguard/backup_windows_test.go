//go:build windows

package steamguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

func TestCreateVerifiedBackupCopiesAndDecryptsEveryRecord(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	const password = "separate Steam Guard password"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.unlockVaultLocked(service.vault, password, false); err != nil {
		t.Fatal(err)
	}
	want := []byte("encrypted account sentinel")
	recordID, err := service.vault.Put("76561198000000000", want)
	if err != nil {
		t.Fatal(err)
	}
	parent := tempDir(t)
	now := time.Date(2026, time.July, 22, 12, 34, 56, 0, time.UTC)
	destination, err := service.createVerifiedBackupAt(parent, password, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(filepath.Dir(destination), parent) || filepath.Base(destination) != backupNamePrefix+"20260722-123456" {
		t.Fatalf("destination = %q", destination)
	}
	copyVault, err := vault.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyVault.Unlock(password, vault.FixedLease); err != nil {
		t.Fatal(err)
	}
	got, err := copyVault.Get(recordID)
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(got)
	if !bytes.Equal(got, want) {
		t.Fatalf("copied record = %q", got)
	}
	settings, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.LastVerifiedBackup != now.Format(time.RFC3339) || settings.LastVerifiedBackupPath != destination {
		t.Fatalf("backup settings = %#v", settings)
	}
}

// A backup leaves the machine and is opened rarely, so it carries a harder
// derivation than the live vault it was copied from.
func TestBackupCarriesHarderKDFThanLiveVault(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	const password = "separate Steam Guard password"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.unlockVaultLocked(service.vault, password, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.vault.Put("76561198000000000", []byte("record")); err != nil {
		t.Fatal(err)
	}
	liveRoot, err := VaultFolderPath()
	if err != nil {
		t.Fatal(err)
	}
	liveMemory := readVaultKDFMemoryKiB(t, liveRoot)

	destination, err := service.createVerifiedBackupAt(tempDir(t), password, "",
		time.Date(2026, time.July, 22, 12, 34, 56, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	backupMemory := readVaultKDFMemoryKiB(t, destination)
	if backupMemory <= liveMemory {
		t.Fatalf("backup KDF memory = %d KiB, live = %d KiB; backup must be harder", backupMemory, liveMemory)
	}
	if backupMemory != service.backupKDFParams().MemoryKiB {
		t.Fatalf("backup KDF memory = %d KiB, want %d", backupMemory, service.backupKDFParams().MemoryKiB)
	}

	// The backup still opens with the unchanged password, under its own
	// parameters, and still decrypts.
	copyVault, err := vault.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyVault.Unlock(password, vault.FixedLease); err != nil {
		t.Fatalf("backup did not unlock under its new parameters: %v", err)
	}
	records, err := copyVault.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("backup holds %d records, want 1", len(records))
	}
	_ = copyVault.Lock()
}

// Restoring makes the backup the live vault, so its deliberately expensive
// parameters have to come back down or every routine unlock pays them.
func TestRestoreRekeysBackupParametersDownToLiveCost(t *testing.T) {
	useSettingsRoot(t)
	const password = "restored Steam Guard password"
	sourceKDF := vault.KDFParams{Algorithm: "argon2id", MemoryKiB: 32 * 1024, Passes: 2, Lanes: 1, KeyBytes: 32}
	source := filepath.Join(tempDir(t), "verified-backup")
	backup, err := vault.Create(source, password, vault.WithKDFParams(sourceKDF))
	if err != nil {
		t.Fatal(err)
	}
	if err := backup.Lock(); err != nil {
		t.Fatal(err)
	}

	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	destination, err := service.restoreVerifiedBackupAt(source, password, "", "")
	if err != nil {
		t.Fatal(err)
	}

	want := service.liveKDFParams().MemoryKiB
	if got := readVaultKDFMemoryKiB(t, destination); got != want {
		t.Fatalf("restored vault KDF memory = %d KiB, want the live cost %d", got, want)
	}
	if want >= sourceKDF.MemoryKiB {
		t.Fatal("test is vacuous: the live cost must be below the source cost")
	}
	// The password still opens it after the rekey.
	if err := service.unlockVaultLocked(service.vault, password, false); err != nil {
		t.Fatalf("restored vault did not unlock after rekey: %v", err)
	}
}

func readVaultKDFMemoryKiB(t *testing.T, root string) uint32 {
	t.Helper()
	active, err := os.ReadFile(filepath.Join(root, "active"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "generation-"+string(active), "header.json"))
	if err != nil {
		t.Fatal(err)
	}
	var h struct {
		Slots []struct {
			Factors []struct {
				Type string `json:"type"`
				KDF  *struct {
					MemoryKiB uint32 `json:"memoryKiB"`
				} `json:"kdf"`
			} `json:"factors"`
		} `json:"slots"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatal(err)
	}
	for _, slot := range h.Slots {
		for _, factor := range slot.Factors {
			if factor.Type == vault.FactorPassword && factor.KDF != nil {
				return factor.KDF.MemoryKiB
			}
		}
	}
	t.Fatal("no password factor found in vault header")
	return 0
}

func TestCreateVerifiedBackupDeletesFailedOutput(t *testing.T) {
	root := useSettingsRoot(t)
	service := newServiceForTest()
	const password = "separate Steam Guard password"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.unlockVaultLocked(service.vault, password, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.vault.Put("76561198000000000", []byte("record")); err != nil {
		t.Fatal(err)
	}
	parent := tempDir(t)
	if _, err := service.createVerifiedBackupAt(parent, "wrong password", "", time.Now()); !errors.Is(err, vault.ErrInvalidPassword) {
		t.Fatalf("wrong password error = %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed backup left entries: %v", entries)
	}
	if _, err := service.createVerifiedBackupAt(filepath.Join(root, "SteamGuard"), password, "", time.Now()); !errors.Is(err, ErrInvalidBackupDestination) {
		t.Fatalf("nested destination error = %v", err)
	}
}

func TestCreateVerifiedBackupAuthenticatesRecoveryWrapper(t *testing.T) {
	useSettingsRoot(t)
	service := NewService()
	t.Cleanup(func() {
		security.SetSteamGuardLifecycleHook(nil)
		_ = security.Lock()
	})
	const appPassword = "separate application password"
	const steamGuardPassword = "separate Steam Guard password"
	if err := security.SetAppPassword(appPassword); err != nil {
		t.Fatal(err)
	}
	if err := security.EnableSavedAccountEncryption(appPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Initialize(steamGuardPassword, appPassword); err != nil {
		t.Fatal(err)
	}
	if err := service.unlockVaultLocked(service.vault, steamGuardPassword, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.vault.Put("76561198000000000", []byte("double-encrypted record")); err != nil {
		t.Fatal(err)
	}
	parent := tempDir(t)
	if _, err := service.createVerifiedBackupAt(parent, steamGuardPassword, "wrong app password", time.Now()); err == nil {
		t.Fatal("wrong app password verified backup")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed backup cleanup = %v, %v", entries, err)
	}
	destination, err := service.createVerifiedBackupAt(parent, steamGuardPassword, appPassword, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	copyVault, err := vault.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyVault.VerifyRecovery(steamGuardPassword, appPassword); err != nil {
		t.Fatal(err)
	}
}

// Backing up rekeys the copy to the deliberately expensive backup parameters,
// which re-derives every factor of each slot. That derivation was password-only,
// so a vault whose way in needed a password and a keyfile could not be backed
// up at all - the feature that protects the vault best broke the safety net.
func TestBackupAndRestoreWorkOnAVaultThatNeedsAKeyfile(t *testing.T) {
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
	if _, err := service.vault.Put("76561198000000001", []byte("combined sentinel")); err != nil {
		t.Fatal(err)
	}

	parent := tempDir(t)
	if _, err := service.createVerifiedBackupAt(parent, password, "", time.Now().UTC()); !errors.Is(err, vault.ErrFactorRequired) {
		t.Fatalf("backed up a combined vault with the password alone: %v", err)
	}
	creds, err := buildVaultCredentials(password, keyfilePath, "")
	if err != nil {
		t.Fatal(err)
	}
	destination, err := service.createVerifiedBackupAtWith(parent, creds, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("backing up with the keyfile: %v", err)
	}

	// The copy keeps the factors it was made with, so restoring needs them too.
	copyVault, err := vault.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyVault.Unlock(password, vault.FixedLease); !errors.Is(err, vault.ErrFactorRequired) {
		t.Fatalf("backup copy opened with the password alone: %v", err)
	}
	if err := copyVault.UnlockWith(creds, vault.FixedLease); err != nil {
		t.Fatalf("backup copy did not open with the keyfile: %v", err)
	}
}
