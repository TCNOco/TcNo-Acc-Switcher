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

	t.Cleanup(func() {
		defaultManager = &manager{}
		SetStatusChangedHook(nil)
	})
	return root
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

	if err := SetAppPassword("secret"); err != nil {
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
	if err := UnlockApp("secret"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAppPassword("wrong"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("RemoveAppPassword(wrong) error = %v, want ErrInvalidPassword", err)
	}
	if err := RemoveAppPassword("secret"); err != nil {
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

func TestSavedAccountVaultRoundTripAndTamperDetection(t *testing.T) {
	root := resetSecurityTest(t)

	const (
		platformKey = "Example"
		uniqueID    = "uid-1"
		accountName = "Account One"
		password    = "secret"
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
