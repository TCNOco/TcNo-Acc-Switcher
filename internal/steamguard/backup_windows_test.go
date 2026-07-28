//go:build windows

package steamguard

import (
	"bytes"
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
	parent := t.TempDir()
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
	parent := t.TempDir()
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
	parent := t.TempDir()
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
