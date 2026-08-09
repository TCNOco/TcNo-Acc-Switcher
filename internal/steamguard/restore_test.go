package steamguard

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// A restore builds the live vault folder in place and deletes it again if a
// later step fails, and it releases the service lock in between to ask a
// security key enrolled in the backup. Opening that folder in the window would
// cache a vault over a directory about to be removed, leaving the process
// convinced a vault exists and refusing to restore or create one until it is
// restarted - so the half-built folder must not be openable at all.
func TestVaultIsNotOpenableDuringARestore(t *testing.T) {
	useSettingsRoot(t)
	const password = "restored Steam Guard password"
	source := createRestoreSource(t, password, "")
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	// Restore the source once so a complete, openable vault exists at the live
	// path: the window this guards is exactly when one does.
	if _, err := service.restoreVerifiedBackupAt(source, password, "", ""); err != nil {
		t.Fatal(err)
	}
	service.vault = nil

	service.mu.Lock()
	service.restoreInProgress = true
	_, exists, err := service.openVaultLocked()
	service.restoreInProgress = false
	cached := service.vault
	service.mu.Unlock()

	if !errors.Is(err, ErrRestoreInProgress) {
		t.Fatalf("opening the vault mid-restore: err = %v, exists = %v, want %v", err, exists, ErrRestoreInProgress)
	}
	if cached != nil {
		t.Fatal("a vault opened mid-restore was cached, and the restore may still delete that folder")
	}
}

// A restore that fails part-way must leave nothing behind that stops the user
// simply trying again.
func TestFailedRestoreCanBeRetried(t *testing.T) {
	useSettingsRoot(t)
	const password = "restored Steam Guard password"
	source := createRestoreSource(t, password, "")
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	if _, err := service.restoreVerifiedBackupAt(source, "not the backup password", "", ""); err == nil {
		t.Fatal("a restore with the wrong password reported success")
	}
	if service.vault != nil {
		t.Fatal("the failed restore left a vault cached over the deleted folder")
	}
	if service.restoreInProgress {
		t.Fatal("the failed restore left the in-progress flag set")
	}
	if _, err := service.restoreVerifiedBackupAt(source, password, "", ""); err != nil {
		t.Fatalf("retry after a failed restore: %v", err)
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
	source := filepath.Join(tempDir(t), "verified-backup")
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

func TestInspectRestoreBackupReportsOuterLayer(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	plain := createRestoreSource(t, "plain backup password", "")
	info, err := service.InspectRestoreBackup(plain)
	if err != nil {
		t.Fatal(err)
	}
	if info.HasOuterLayer {
		t.Fatal("single-layer backup reported an outer layer")
	}

	wrapped := createRestoreSource(t, "wrapped backup password", "backup recovery password")
	info, err = service.InspectRestoreBackup(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasOuterLayer {
		t.Fatal("recovery-wrapped backup reported no outer layer")
	}
}

// The folder belongs to the user, so looking at it must not change it: Open
// would run journal recovery and harden the directory, Inspect must not.
func TestInspectRestoreBackupLeavesTheFolderUntouched(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	source := createRestoreSource(t, "untouched backup password", "")
	before := treeDigest(t, source)
	if _, err := service.InspectRestoreBackup(source); err != nil {
		t.Fatal(err)
	}
	if after := treeDigest(t, source); after != before {
		t.Fatalf("inspection modified the backup folder:\n%s\n%s", before, after)
	}
}

func TestInspectRestoreBackupRejectsUnrelatedFolder(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	folder := tempDir(t)
	if err := os.WriteFile(filepath.Join(folder, "notes.txt"), []byte("not a vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InspectRestoreBackup(folder); err == nil {
		t.Fatal("a folder without a vault was accepted as a backup")
	}
}

// treeDigest fingerprints every file path, size and content under root.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			entries = append(entries, "d "+filepath.ToSlash(rel))
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		entries = append(entries, "f "+filepath.ToSlash(rel)+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	return strings.Join(entries, "\n")
}
