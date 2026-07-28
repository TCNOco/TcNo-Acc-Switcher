package steamguard

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

func TestRestoreVerifiedBackupIntoEmptyVault(t *testing.T) {
	useSettingsRoot(t)
	const password = "restored Steam Guard password"
	source := createRestoreSource(t, password, "")
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	destination, err := service.restoreVerifiedBackupAt(source, password, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if destination != filepath.Join(filepath.Dir(destination), "SteamGuard") {
		t.Fatalf("restore destination = %q", destination)
	}
	grant := issueSensitiveGrant(t, service, qrTestAccountID, "request-restored-vault-0001")
	view, err := service.UnlockAccount(qrTestAccountID, password, false, grant.Capability)
	if err != nil || view.AccountName != "restored_account" || len(view.Code) != 5 {
		t.Fatalf("restored view = %#v, %v", view, err)
	}
}

func TestRestoreOuterBackupWithoutCurrentAppPassword(t *testing.T) {
	useSettingsRoot(t)
	const (
		password          = "restored Steam Guard password"
		backupAppPassword = "old app recovery password"
	)
	source := createRestoreSource(t, password, backupAppPassword)
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	if _, err := service.restoreVerifiedBackupAt(source, password, backupAppPassword, ""); err != nil {
		t.Fatal(err)
	}
	if service.vault.HasRecoveryWrapper() {
		t.Fatal("outer layer remained without a current app password")
	}
	if err := service.vault.Unlock(password, vault.FixedLease); err != nil {
		t.Fatal(err)
	}
}

func createRestoreSource(t *testing.T, password, recoveryPassword string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "verified-backup")
	backup, err := vault.Create(source, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := backup.Unlock(password, vault.FixedLease); err != nil {
		t.Fatal(err)
	}
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x24}, 20)),
		DeviceID:       "android:01234567-89ab-cdef-0123-456789abcdef",
		AccountName:    "restored_account", FullyEnrolled: true,
		Session: &mafile.SessionData{SteamID: 76561198000000000},
	}
	plaintext, err := mafile.ExportPlaintext(account, mafile.ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backup.PutRecord(qrTestAccountID, plaintext); err != nil {
		t.Fatal(err)
	}
	if recoveryPassword != "" {
		outerKey := bytes.Repeat([]byte{0x5a}, 32)
		if err := backup.EnableOuterWithRecovery(outerKey, recoveryPassword); err != nil {
			t.Fatal(err)
		}
	}
	if err := backup.Lock(); err != nil {
		t.Fatal(err)
	}
	return source
}
