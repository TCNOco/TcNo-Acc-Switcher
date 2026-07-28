//go:build windows

package steamguard

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

func mergeTestPlaintext(t *testing.T, accountName string, steamID uint64) []byte {
	t.Helper()
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x24}, 20)),
		DeviceID:       "android:01234567-89ab-cdef-0123-456789abcdef",
		AccountName:    accountName, FullyEnrolled: true,
		Session: &mafile.SessionData{SteamID: steamID},
	}
	plaintext, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	return plaintext
}

func mergeTestBackup(t *testing.T, password string, accounts map[uint64]string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "verified-backup")
	backup, err := vault.Create(source, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := backup.Unlock(password, vault.FixedLease); err != nil {
		t.Fatal(err)
	}
	for steamID, name := range accounts {
		if _, err := backup.PutRecord(strconv.FormatUint(steamID, 10), mergeTestPlaintext(t, name, steamID)); err != nil {
			t.Fatal(err)
		}
	}
	if err := backup.Lock(); err != nil {
		t.Fatal(err)
	}
	return source
}

func TestRestoreMergeReplacesOnlySelectedAccounts(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	const password = "merge Steam Guard password"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.unlockVaultLocked(service.vault, password, false); err != nil {
		t.Fatal(err)
	}
	const conflictID, freshID = uint64(76561198000000001), uint64(76561198000000002)
	conflict64 := strconv.FormatUint(conflictID, 10)
	if _, err := service.vault.PutRecord(conflict64, mergeTestPlaintext(t, "live_name", conflictID)); err != nil {
		t.Fatal(err)
	}

	source := mergeTestBackup(t, password, map[uint64]string{conflictID: "backup_name", freshID: "backup_fresh"})
	stage, err := service.stageRestoreMerge(source)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.planStagedRestoreMerge(stage, password, "", "")
	if err != nil || plan.State != "ok" || len(plan.Accounts) != 2 {
		t.Fatalf("plan = %#v, %v", plan, err)
	}
	byID := map[string]RestoreMergeAccount{}
	for _, account := range plan.Accounts {
		byID[account.SteamID64] = account
	}
	if !byID[conflict64].Exists || byID[conflict64].AccountName != "backup_name" {
		t.Fatalf("conflict account = %#v", byID[conflict64])
	}
	if byID[strconv.FormatUint(freshID, 10)].Exists {
		t.Fatalf("fresh account marked existing: %#v", byID)
	}

	// Only the conflicting account is brought across; the new one is left behind.
	result, err := service.CommitRestoreMerge(password, "", "", "", []string{conflict64})
	if err != nil {
		t.Fatal(err)
	}
	if result.Replaced != 1 || result.Added != 0 || !result.CapabilityRefreshRequired {
		t.Fatalf("result = %#v", result)
	}
	if result.SafetyBackupPath == "" {
		t.Fatal("no safety backup path")
	}
	if _, err := os.Stat(filepath.Join(result.SafetyBackupPath, "active")); err != nil {
		t.Fatalf("safety backup is not a vault copy: %v", err)
	}

	records, err := service.vault.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].SteamID64 != conflict64 {
		t.Fatalf("live records = %#v", records)
	}
	plaintext, err := service.vault.Get(records[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(plaintext)
	parsed, err := mafile.ParsePlaintext(plaintext)
	if err != nil || parsed.Account.AccountName != "backup_name" {
		t.Fatalf("merged account = %#v, %v", parsed.Account.AccountName, err)
	}
	if service.restoreMergeStage != "" {
		t.Fatal("stage was not discarded after commit")
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatalf("stage folder survives: %v", err)
	}
}

// A backup made before a password change unlocks with its own password, and
// the mismatch is reported as a retryable state that keeps the stage.
func TestRestoreMergeReportsBackupPasswordMismatch(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	const password = "merge Steam Guard password"
	const oldPassword = "password before the change"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	source := mergeTestBackup(t, oldPassword, map[uint64]string{76561198000000003: "old_backup"})
	stage, err := service.stageRestoreMerge(source)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.planStagedRestoreMerge(stage, password, "", "")
	if err != nil || plan.State != "backup_password" {
		t.Fatalf("plan = %#v, %v", plan, err)
	}
	if service.restoreMergeStage == "" {
		t.Fatal("stage discarded on password mismatch")
	}
	plan, err = service.planStagedRestoreMerge(stage, password, oldPassword, "")
	if err != nil || plan.State != "ok" || len(plan.Accounts) != 1 {
		t.Fatalf("retry plan = %#v, %v", plan, err)
	}
	if err := service.CancelRestoreMerge(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatalf("stage folder survives cancel: %v", err)
	}
}
