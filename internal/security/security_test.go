package security

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
)

func resetSecurityTest(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	paths.ResetForTest(root)
	defaultManager = &manager{}
	SetStatusChangedHook(nil)
	SetSteamGuardLifecycleHook(nil)

	t.Cleanup(func() {
		defaultManager = &manager{}
		SetStatusChangedHook(nil)
		SetSteamGuardLifecycleHook(nil)
	})
	return root
}

type testSteamGuardLifecycle struct {
	enabled      bool
	enableCalls  int
	disableCalls int
	changeCalls  int
	revokeCalls  int
	key          []byte
	password     string
}

func (h *testSteamGuardLifecycle) EnableOuter(key []byte, recoveryPassword string) error {
	h.enableCalls++
	h.enabled = true
	h.key = append(h.key[:0], key...)
	h.password = recoveryPassword
	return nil
}

func (h *testSteamGuardLifecycle) DisableOuter(key []byte) error {
	h.disableCalls++
	if len(h.key) != 0 && !bytes.Equal(h.key, key) {
		return errors.New("outer key changed")
	}
	h.enabled = false
	return nil
}

func (h *testSteamGuardLifecycle) ChangeOuterPassword(oldPassword, newPassword string) error {
	h.changeCalls++
	if h.enabled && h.password != oldPassword {
		return errors.New("outer recovery password changed unexpectedly")
	}
	h.password = newPassword
	return nil
}

func (h *testSteamGuardLifecycle) RevokeLeases() error {
	h.revokeCalls++
	return nil
}

func TestSteamGuardPurposeKeySurvivesPasswordChangeAndWipes(t *testing.T) {
	resetSecurityTest(t)
	hook := &testSteamGuardLifecycle{}
	SetSteamGuardLifecycleHook(hook)
	if err := SetAppPassword("old secret password"); err != nil {
		t.Fatal(err)
	}
	if err := EnableSavedAccountEncryption("old secret password"); err != nil {
		t.Fatal(err)
	}
	before, err := DeriveSteamGuardOuterKey()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, defaultManager.masterKey) {
		t.Fatal("purpose key exposed the app master key")
	}
	if err := ChangePassword("old secret password", "new secret password"); err != nil {
		t.Fatal(err)
	}
	after, err := DeriveSteamGuardOuterKey()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("password change changed the Steam Guard purpose key")
	}
	if hook.revokeCalls != 1 {
		t.Fatalf("revoke calls = %d, want 1", hook.revokeCalls)
	}
	if hook.changeCalls != 1 || hook.password != "new secret password" {
		t.Fatalf("outer recovery password was not changed: %+v", hook)
	}
	WipeSecret(before)
	WipeSecret(after)
	if !bytes.Equal(before, make([]byte, len(before))) || !bytes.Equal(after, make([]byte, len(after))) {
		t.Fatal("WipeSecret left purpose-key bytes reachable")
	}
}

func TestChangePasswordRollsBackSteamGuardRecoveryOnSaveFailure(t *testing.T) {
	resetSecurityTest(t)
	hook := &testSteamGuardLifecycle{}
	SetSteamGuardLifecycleHook(hook)
	if err := SetAppPassword("old secret password"); err != nil {
		t.Fatal(err)
	}
	if err := EnableSavedAccountEncryption("old secret password"); err != nil {
		t.Fatal(err)
	}
	defaultManager.saveSecurityFile = func(securityFile) error { return errors.New("save failed") }
	if err := ChangePassword("old secret password", "new secret password"); err == nil {
		t.Fatal("ChangePassword succeeded despite save failure")
	}
	if hook.changeCalls != 2 || hook.password != "old secret password" {
		t.Fatalf("recovery rollback = %+v", hook)
	}
}

func TestSetAppPasswordDoesNotEnableSteamGuardOuterWhenEncryptionIsOff(t *testing.T) {
	resetSecurityTest(t)
	hook := &testSteamGuardLifecycle{}
	SetSteamGuardLifecycleHook(hook)
	defaultManager.saveSecurityFile = func(securityFile) error { return errors.New("save failed") }
	if err := SetAppPassword("valid test password"); err == nil {
		t.Fatal("SetAppPassword succeeded despite save failure")
	}
	if hook.enabled || hook.enableCalls != 0 || hook.disableCalls != 0 {
		t.Fatalf("outer layer changed while saved-data encryption was off: %+v", hook)
	}
	if _, ok, err := loadSecurityFile(); err != nil || ok {
		t.Fatalf("security file exists after failed setup: found=%v err=%v", ok, err)
	}
}

func TestRemoveAppPasswordRestoresSteamGuardOuterOnRemoveFailure(t *testing.T) {
	resetSecurityTest(t)
	hook := &testSteamGuardLifecycle{}
	SetSteamGuardLifecycleHook(hook)
	if err := SetAppPassword("valid test password"); err != nil {
		t.Fatal(err)
	}
	if err := EnableSavedAccountEncryption("valid test password"); err != nil {
		t.Fatal(err)
	}
	defaultManager.removeFile = func(string) error { return errors.New("remove failed") }
	if err := RemoveAppPassword("valid test password"); err == nil {
		t.Fatal("RemoveAppPassword succeeded despite remove failure")
	}
	if !hook.enabled || hook.disableCalls != 1 || hook.enableCalls != 2 {
		t.Fatalf("hook after rollback = %+v", hook)
	}
	if st, err := GetStatus(); err != nil || !st.AppPasswordSet {
		t.Fatalf("password disappeared after failed removal: status=%+v err=%v", st, err)
	}
}

func TestLockRevokesSteamGuardLeases(t *testing.T) {
	resetSecurityTest(t)
	hook := &testSteamGuardLifecycle{}
	SetSteamGuardLifecycleHook(hook)
	if err := SetAppPassword("valid test password"); err != nil {
		t.Fatal(err)
	}
	if err := EnableSavedAccountEncryption("valid test password"); err != nil {
		t.Fatal(err)
	}
	if err := Lock(); err != nil {
		t.Fatal(err)
	}
	if hook.revokeCalls != 1 {
		t.Fatalf("revoke calls = %d, want 1", hook.revokeCalls)
	}
	if err := UnlockApp("valid test password"); err != nil {
		t.Fatal(err)
	}
	if hook.enableCalls != 2 || !hook.enabled {
		t.Fatalf("unlock did not enforce the outer layer: %+v", hook)
	}
}

func TestAppPasswordLifecycle(t *testing.T) {
	resetSecurityTest(t)

	st, err := GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if st.AppPasswordSet || st.AppLocked {
		t.Fatalf("initial status = %+v, want no password and unlocked", st)
	}

	if err := SetAppPassword("valid test password"); err != nil {
		t.Fatal(err)
	}
	st, err = GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !st.AppPasswordSet || st.AppLocked {
		t.Fatalf("after setup status = %+v, want password set and current session unlocked", st)
	}

	defaultManager = &manager{}
	st, err = GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !st.AppPasswordSet || !st.AppLocked {
		t.Fatalf("after restart status = %+v, want locked", st)
	}

	if err := UnlockApp("wrong"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("UnlockApp(wrong) error = %v, want ErrInvalidPassword", err)
	}
	if err := UnlockApp("valid test password"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAppPassword("wrong"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("RemoveAppPassword(wrong) error = %v, want ErrInvalidPassword", err)
	}
	if err := RemoveAppPassword("valid test password"); err != nil {
		t.Fatal(err)
	}
	st, err = GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if st.AppPasswordSet || st.AppLocked || st.SavedAccountDataEncrypted {
		t.Fatalf("after removal status = %+v, want no password, unlocked, unencrypted", st)
	}
}

func TestPasswordSetupAndChangeRejectEmptyPassword(t *testing.T) {
	resetSecurityTest(t)

	if err := SetAppPassword(""); !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("SetAppPassword(empty) error = %v, want ErrEmptyPassword", err)
	}
	if st, err := GetStatus(); err != nil {
		t.Fatal(err)
	} else if st.AppPasswordSet {
		t.Fatalf("status after rejected setup = %+v, want no password", st)
	}

	if err := SetAppPassword("old secret password"); err != nil {
		t.Fatal(err)
	}
	if err := ChangePassword("old secret password", ""); !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("ChangePassword(empty new password) error = %v, want ErrEmptyPassword", err)
	}
	if err := Lock(); err != nil {
		t.Fatal(err)
	}
	if err := UnlockApp("old secret password"); err != nil {
		t.Fatalf("old password stopped working after rejected change: %v", err)
	}
}

func TestNewAppPasswordPolicy(t *testing.T) {
	resetSecurityTest(t)

	if err := SetAppPassword("x"); err != nil {
		t.Fatalf("SetAppPassword(one character) error = %v", err)
	}
	if err := VerifyAppPassword("x"); err != nil {
		t.Fatalf("VerifyAppPassword(one character) error = %v", err)
	}
}

func TestLegacyPasswordUnlockCompatibilityAndSteamGuardEligibility(t *testing.T) {
	resetSecurityTest(t)
	const legacyPassword = "short"
	writeLegacySecurityFile(t, legacyPassword)

	if err := UnlockApp(legacyPassword); err != nil {
		t.Fatalf("legacy password no longer unlocks app: %v", err)
	}
	if err := VerifyAppPassword(legacyPassword); err != nil {
		t.Fatalf("legacy password was rejected for Steam Guard outer layer: %v", err)
	}
	if err := ChangePassword(legacyPassword, "replacement app password"); err != nil {
		t.Fatalf("legacy password could not be changed: %v", err)
	}
	if err := VerifyAppPassword("replacement app password"); err != nil {
		t.Fatalf("eligible replacement was rejected: %v", err)
	}
}

func writeLegacySecurityFile(t *testing.T, password string) {
	t.Helper()
	salt, err := randomBytes(16)
	if err != nil {
		t.Fatal(err)
	}
	kdf := KDFParams{Algorithm: "argon2id", Time: 1, MemoryKB: 8 * 1024, Threads: 1, KeyLen: vaultKeyBytes}
	derived := deriveKey(password, salt, kdf)
	defer wipeBytes(derived)
	master, err := randomBytes(vaultKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	defer wipeBytes(master)
	verifierNonce, verifierCipher, err := sealWithKey(derived, []byte("tcno-security-ok"), []byte(securityVerifierAAD))
	if err != nil {
		t.Fatal(err)
	}
	wrapNonce, wrapped, err := sealWithKey(derived, master, []byte(wrappedKeyAAD))
	if err != nil {
		t.Fatal(err)
	}
	if err := saveSecurityFile(securityFile{
		Version:                   securityVersion,
		KDF:                       kdf,
		Salt:                      encode(salt),
		VerifierNonce:             encode(verifierNonce),
		VerifierCiphertext:        encode(verifierCipher),
		WrappedVaultKeyNonce:      encode(wrapNonce),
		WrappedVaultKeyCiphertext: encode(wrapped),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestChangePasswordPreservesMasterKeyAndAccountBlobs(t *testing.T) {
	root := resetSecurityTest(t)

	const (
		oldPassword = "old secret password"
		newPassword = "new secret password"
		platformKey = "Example"
		uniqueID    = "uid-1"
		accountName = "Account One"
	)
	if err := SetAppPassword(oldPassword); err != nil {
		t.Fatal(err)
	}
	masterBefore, err := defaultManager.unlockedMasterKey()
	if err != nil {
		t.Fatal(err)
	}
	accountDir := filepath.Join(root, "account")
	writeTestFile(t, filepath.Join(accountDir, "data.bin"), []byte("encrypted account payload"))
	if err := writeAccountBlob(masterBefore, platformKey, uniqueID, accountName, accountDir); err != nil {
		t.Fatal(err)
	}
	if err := updateSecurityFile(func(sf *securityFile) error {
		sf.SavedAccountDataEncrypted = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	blobPath, err := accountBlobPath(platformKey, uniqueID)
	if err != nil {
		t.Fatal(err)
	}
	blobBefore := readTestFile(t, blobPath)

	if err := Lock(); err != nil {
		t.Fatal(err)
	}
	if err := ChangePassword("wrong", newPassword); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("ChangePassword(wrong old password) error = %v, want ErrInvalidPassword", err)
	}
	if err := RequireUnlocked(); !errors.Is(err, ErrLocked) {
		t.Fatalf("RequireUnlocked after rejected change error = %v, want ErrLocked", err)
	}
	if err := ChangePassword(oldPassword, newPassword); err != nil {
		t.Fatal(err)
	}

	masterAfter, err := defaultManager.unlockedMasterKey()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(masterAfter, masterBefore) {
		t.Fatal("password change replaced the random master key")
	}
	if blobAfter := readTestFile(t, blobPath); !bytes.Equal(blobAfter, blobBefore) {
		t.Fatal("password change rewrote the encrypted account blob")
	}
	if !AccountBlobValid(platformKey, uniqueID) {
		t.Fatal("account blob is not valid after password change")
	}
	if sf, ok, err := loadSecurityFile(); err != nil {
		t.Fatal(err)
	} else if !ok || sf.Version != securityVersion || !sf.SavedAccountDataEncrypted {
		t.Fatalf("security file metadata after password change = %+v, found=%v", sf, ok)
	}

	if err := Lock(); err != nil {
		t.Fatal(err)
	}
	if err := UnlockApp(oldPassword); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("UnlockApp(old password) error = %v, want ErrInvalidPassword", err)
	}
	if err := UnlockApp(newPassword); err != nil {
		t.Fatalf("UnlockApp(new password) error = %v", err)
	}
}

func TestLockWipesMasterKeyAndRequiresUnlock(t *testing.T) {
	resetSecurityTest(t)

	if err := SetAppPassword("valid test password"); err != nil {
		t.Fatal(err)
	}
	keyBuffer := defaultManager.masterKey
	if len(keyBuffer) == 0 {
		t.Fatal("setup did not retain an unlocked master key")
	}

	if err := Lock(); err != nil {
		t.Fatal(err)
	}
	if len(defaultManager.masterKey) != 0 {
		t.Fatal("Lock retained the active master key")
	}
	if !bytes.Equal(keyBuffer, make([]byte, len(keyBuffer))) {
		t.Fatal("Lock did not wipe the replaced master-key buffer")
	}
	if err := RequireUnlocked(); !errors.Is(err, ErrLocked) {
		t.Fatalf("RequireUnlocked after Lock error = %v, want ErrLocked", err)
	}
	if err := UnlockApp("wrong"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("UnlockApp(wrong) error = %v, want ErrInvalidPassword", err)
	}
	if err := RequireUnlocked(); !errors.Is(err, ErrLocked) {
		t.Fatalf("RequireUnlocked after failed unlock error = %v, want ErrLocked", err)
	}
	if err := UnlockApp("valid test password"); err != nil {
		t.Fatal(err)
	}
	if err := RequireUnlocked(); err != nil {
		t.Fatalf("RequireUnlocked after successful unlock error = %v", err)
	}
}

func TestSavedAccountVaultRoundTripAndTamperDetection(t *testing.T) {
	root := resetSecurityTest(t)

	const (
		platformKey = "Example"
		uniqueID    = "uid-1"
		accountName = "Account One"
		password    = "valid test password"
	)
	accountDir := filepath.Join(root, "LoginCache", platformKey, paths.SanitizePathSegment(accountName))
	writeTestFile(t, filepath.Join(accountDir, "nested", "data.bin"), []byte{0, 1, 2, 3, 255})
	writeTestFile(t, filepath.Join(accountDir, "reg.json"), []byte(`{"HKCU\\Software\\Example":{"v":"uid-1","t":"REG_SZ"}}`))
	if err := os.MkdirAll(filepath.Join(accountDir, "nested", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(root, "LoginCache", platformKey, "ids.json"), map[string]any{
		"ids":      map[string]string{uniqueID: accountName},
		"lastused": map[string]string{uniqueID: "2026-06-28T00:00:00Z"},
		"notes":    map[string]string{uniqueID: "metadata stays plaintext"},
	})

	if err := SetAppPassword(password); err != nil {
		t.Fatal(err)
	}
	if err := EnableSavedAccountEncryption(password); err != nil {
		t.Fatal(err)
	}
	st, err := GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !st.SavedAccountDataEncrypted {
		t.Fatalf("status = %+v, want saved account data encrypted", st)
	}
	if _, err := os.Stat(accountDir); !os.IsNotExist(err) {
		t.Fatalf("plaintext account dir still exists or stat failed: %v", err)
	}
	blobPath, err := accountBlobPath(platformKey, uniqueID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("encrypted blob missing: %v", err)
	}
	if !AccountBlobValid(platformKey, uniqueID) {
		t.Fatal("encrypted blob should validate")
	}

	restoreDir, cleanup, err := AccountRestoreDir(platformKey, uniqueID, accountName, accountDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(restoreDir, "nested", "data.bin")); !bytes.Equal(got, []byte{0, 1, 2, 3, 255}) {
		t.Fatalf("restored binary = %v", got)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "nested", "empty")); err != nil {
		t.Fatalf("empty directory was not restored: %v", err)
	}
	cleanup()
	if _, err := os.Stat(restoreDir); !os.IsNotExist(err) {
		t.Fatalf("restore staging still exists or stat failed: %v", err)
	}

	if err := DisableSavedAccountEncryption(password); err != nil {
		t.Fatal(err)
	}
	st, err = GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if st.SavedAccountDataEncrypted {
		t.Fatalf("status = %+v, want saved account data decrypted", st)
	}
	if got := readTestFile(t, filepath.Join(accountDir, "nested", "data.bin")); !bytes.Equal(got, []byte{0, 1, 2, 3, 255}) {
		t.Fatalf("decrypted binary = %v", got)
	}
	if _, err := os.Stat(blobPath); !os.IsNotExist(err) {
		t.Fatalf("blob still exists after disable or stat failed: %v", err)
	}

	if err := EnableSavedAccountEncryption(password); err != nil {
		t.Fatal(err)
	}
	tamperBlobCiphertext(t, blobPath)
	if AccountBlobValid(platformKey, uniqueID) {
		t.Fatal("tampered blob should not validate")
	}
	if _, cleanup, err := AccountRestoreDir(platformKey, uniqueID, accountName, accountDir); err == nil {
		cleanup()
		t.Fatal("tampered blob restore succeeded")
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTestJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, append(data, '\n'))
}

func tamperBlobCiphertext(t *testing.T, path string) {
	t.Helper()
	var blob encryptedAccountBlob
	data := readTestFile(t, path)
	if err := json.Unmarshal(data, &blob); err != nil {
		t.Fatal(err)
	}
	if blob.Ciphertext == "" {
		t.Fatal("blob ciphertext is empty")
	}
	last := blob.Ciphertext[len(blob.Ciphertext)-1]
	if last == 'A' {
		last = 'B'
	} else {
		last = 'A'
	}
	blob.Ciphertext = blob.Ciphertext[:len(blob.Ciphertext)-1] + string(last)
	writeTestJSON(t, path, blob)
}
