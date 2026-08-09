package steamguard

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/securemem"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"golang.org/x/crypto/pbkdf2"
)

type serviceTestClock struct{ now time.Time }

func (c *serviceTestClock) Now() time.Time      { return c.now }
func (c *serviceTestClock) Add(d time.Duration) { c.now = c.now.Add(d) }

func TestServiceImportRestartLockAndCode(t *testing.T) {
	root := useSettingsRoot(t)
	clock := &serviceTestClock{now: time.Unix(1_700_000_000, 0)}
	service := newServiceForTest(vault.WithClock(clock))
	service.setMainContentProtectionFn = func(bool) error { return nil }
	const password = "correct horse battery staple"
	vaultPath, err := service.Initialize(password, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "SteamGuard"); vaultPath != want {
		t.Fatalf("vault path = %q, want %q", vaultPath, want)
	}

	secret := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 20))
	account := mafile.Account{
		SharedSecret:   secret,
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x24}, 20)),
		DeviceID:       "android:01234567-89ab-cdef-0123-456789abcdef",
		AccountName:    "test_account",
		FullyEnrolled:  true,
		Session: &mafile.SessionData{
			SteamID: 76561198000000000, AccessToken: "secret-access-token", RefreshToken: "secret-refresh-token",
		},
	}
	plain, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(tempDir(t), "76561198000000000.maFile")
	if err := os.WriteFile(source, plain, 0o600); err != nil {
		t.Fatal(err)
	}

	results, err := service.ImportPlaintext([]string{source}, password, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Imported || results[0].SteamID64 != "76561198000000000" {
		t.Fatalf("results = %#v", results)
	}
	results, err = service.ImportPlaintext([]string{source}, "", false)
	if err != nil || len(results) != 1 || !results[0].Imported {
		t.Fatalf("unlocked import = %#v, err = %v", results, err)
	}
	if err := service.SetRememberPasswordForSession(true); err != nil {
		t.Fatal(err)
	}
	clock.Add(vault.FixedLeaseLength + time.Second)
	results, err = service.ImportPlaintext([]string{source}, "", false)
	if err != nil || len(results) != 1 || !results[0].Imported {
		t.Fatalf("session import = %#v, err = %v", results, err)
	}
	unchanged, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(unchanged, plain) {
		t.Fatalf("source changed, err = %v", err)
	}
	has, pending := registry.Status("76561198000000000")
	if !has || pending {
		t.Fatalf("registry status = %v, %v", has, pending)
	}
	grant := issueSensitiveGrant(t, service, "76561198000000000", "request-import-0001")
	if _, err := service.GetCode("76561198000000001", grant.Capability); !errors.Is(err, capability.ErrInvalidCapability) {
		t.Fatalf("cross-account capability error = %v", err)
	}
	view, err := service.GetCode("76561198000000000", grant.Capability)
	if err != nil || view == nil || len(view.Code) != 5 || view.AccountName != "test_account" {
		t.Fatalf("code view = %#v, err = %v", view, err)
	}
	clipboard := &clipboardRecorder{}
	service.clipboard = clipboard
	if err := service.CopyCode("76561198000000000", grant.Capability); err != nil {
		t.Fatal(err)
	}
	if len(clipboard.code) != 5 || clipboard.lifetime <= 0 || clipboard.lifetime > 31*time.Second {
		t.Fatalf("clipboard copy = %q for %s", clipboard.code, clipboard.lifetime)
	}
	if runtime.GOOS == "windows" {
		exportPath := filepath.Join(tempDir(t), "export.maFile")
		if err := exportAccountToPath(service.vault, "76561198000000000", exportPath, false, ""); err != nil {
			t.Fatal(err)
		}
		exported, err := os.ReadFile(exportPath)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(exported, []byte("secret-access-token")) || bytes.Contains(exported, []byte("secret-refresh-token")) {
			t.Fatal("default export leaked session tokens")
		}
		if _, err := mafile.ParsePlaintext(exported); err != nil {
			t.Fatalf("export is not canonical maFile JSON: %v", err)
		}
		if err := exportAccountToPath(service.vault, "76561198000000000", exportPath, false, ""); !errors.Is(err, os.ErrExist) {
			t.Fatalf("overwrite error = %v", err)
		}

		// Encrypted export: the body is ciphertext and the salt and IV go to the
		// manifest beside it, which is the only place SDA looks for them.
		encryptedDir := tempDir(t)
		encryptedPath := filepath.Join(encryptedDir, "76561198000000000.maFile")
		if err := exportAccountToPath(service.vault, "76561198000000000", encryptedPath, false, "mafile-password"); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(encryptedPath)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, []byte("account_name")) {
			t.Fatal("encrypted export wrote readable maFile JSON")
		}
		manifest, err := os.ReadFile(filepath.Join(encryptedDir, "manifest.json"))
		if err != nil {
			t.Fatalf("no manifest beside the encrypted export: %v", err)
		}
		result, err := mafile.ImportLegacyEncrypted(body, manifest, "76561198000000000.maFile", "mafile-password")
		if err != nil {
			t.Fatalf("encrypted export did not import back: %v", err)
		}
		if result.Account.AccountName == "" {
			t.Fatalf("decrypted account = %#v", result.Account)
		}

		// An existing manifest lists every account SDA knows about; overwriting it
		// would be unrecoverable, so the export reports rather than replaces it.
		secondPath := filepath.Join(encryptedDir, "second.maFile")
		err = exportAccountToPath(service.vault, "76561198000000000", secondPath, false, "mafile-password")
		if !errors.Is(err, ErrExportManifestExists) {
			t.Fatalf("second encrypted export error = %v", err)
		}
		kept, readErr := os.ReadFile(filepath.Join(encryptedDir, "manifest.json"))
		if readErr != nil || !bytes.Equal(kept, manifest) {
			t.Fatal("the existing manifest was modified")
		}
		if _, statErr := os.Stat(secondPath); statErr != nil {
			t.Fatal("the maFile itself should still be written")
		}
	}

	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	grant = issueSensitiveGrant(t, service, "76561198000000000", "request-locked-0001")
	if view, err := service.GetCode("76561198000000000", grant.Capability); err != nil || view != nil {
		t.Fatalf("locked code = %#v, err = %v", view, err)
	}
	if _, err := service.UnlockAccount("76561198000000000", "wrong", false, grant.Capability); err == nil {
		t.Fatal("wrong password unlocked vault")
	}

	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	restarted := newServiceForTest()
	restarted.setMainContentProtectionFn = func(bool) error { return nil }
	restartGrant := issueSensitiveGrant(t, restarted, "76561198000000000", "request-restart-0001")
	viewValue, err := restarted.UnlockAccount("76561198000000000", password, false, restartGrant.Capability)
	if err != nil || len(viewValue.Code) != 5 {
		t.Fatalf("restart unlock = %#v, err = %v", viewValue, err)
	}

	if err := filepath.Walk(vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(secret)) || bytes.Contains(data, []byte("test_account")) {
			t.Fatalf("plaintext sentinel found in %s", filepath.Base(path))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

type unavailableLeaseProtector struct{}

func (unavailableLeaseProtector) Store([]byte) (securemem.Handle, error) {
	return nil, securemem.ErrUnavailable
}

func TestUnlockAccountFallsBackToOneOperationWithoutRetainingLease(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	const password = "one operation service password"
	const steamID = "76561198000000401"
	if _, err := service.Initialize(password, ""); err != nil {
		t.Fatal(err)
	}
	seedServiceAccount(t, service, password, steamID, "fallback_account")
	reopenServiceWithUnavailableLease(service)

	grant := issueSensitiveGrant(t, service, steamID, "request-one-operation-0001")
	view, err := service.UnlockAccount(steamID, password, true, grant.Capability)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Code) != 5 || view.AccountName != "fallback_account" || view.UnlockPersistence != UnlockPersistenceOneOperation {
		t.Fatalf("one-operation view = %#v", view)
	}
	if service.vault == nil || !service.vault.IsLocked() {
		t.Fatal("one-operation code retained an unlock lease")
	}
	if next, err := service.GetCode(steamID, grant.Capability); err != nil || next != nil {
		t.Fatalf("locked follow-up code = %#v, err = %v", next, err)
	}
	if err := service.CopyCode(steamID, grant.Capability); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("copy without a retained lease = %v", err)
	}
	if err := service.UnlockSteamGuardVault(steamID, password, true, grant.Capability); !errors.Is(err, ErrRetainedUnlockUnavailable) || !errors.Is(err, vault.ErrOneOperationRequired) {
		t.Fatalf("enrollment unlock fallback status = %v", err)
	}
}

func TestUnlockAccountOneOperationFallbackSupportsOuterEncryption(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	t.Cleanup(func() { _ = security.Lock() })
	const appPassword = "one operation app password"
	const password = "one operation vault password"
	const steamID = "76561198000000402"
	if err := security.SetAppPassword(appPassword); err != nil {
		t.Fatal(err)
	}
	if err := security.EnableSavedAccountEncryption(appPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Initialize(password, appPassword); err != nil {
		t.Fatal(err)
	}
	seedServiceAccount(t, service, password, steamID, "outer_fallback_account")
	reopenServiceWithUnavailableLease(service)

	grant := issueSensitiveGrant(t, service, steamID, "request-outer-fallback-0001")
	view, err := service.UnlockAccount(steamID, password, false, grant.Capability)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Code) != 5 || view.UnlockPersistence != UnlockPersistenceOneOperation {
		t.Fatalf("outer one-operation view = %#v", view)
	}
	if service.vault == nil || !service.vault.IsLocked() {
		t.Fatal("outer one-operation code retained an unlock lease")
	}
}

func seedServiceAccount(t *testing.T, service *Service, password, steamID, accountName string) {
	t.Helper()
	if err := service.unlockVaultLocked(service.vault, password, false); err != nil {
		t.Fatal(err)
	}
	parsedSteamID, err := strconv.ParseUint(steamID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x53}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x35}, 20)),
		DeviceID:       "android:21234567-89ab-cdef-0123-456789abcdef",
		AccountName:    accountName,
		FullyEnrolled:  true,
		Session:        &mafile.SessionData{SteamID: parsedSteamID},
	}
	plain, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(plain)
	if _, err := service.vault.PutRecord(steamID, plain); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
}

func reopenServiceWithUnavailableLease(service *Service) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.vault = nil
	service.vaultOptions = append(service.vaultOptions, vault.WithSecureMemory(unavailableLeaseProtector{}))
}

type clipboardRecorder struct {
	code     string
	lifetime time.Duration
}

func (c *clipboardRecorder) Copy(code string, lifetime time.Duration) error {
	c.code = code
	c.lifetime = lifetime
	return nil
}

func (c *clipboardRecorder) Clear() (bool, error) {
	c.code = ""
	return true, nil
}

func (c *clipboardRecorder) Close() error { return nil }

func TestServiceImportFailsClosedPerFile(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	if _, err := service.Initialize("correct horse battery staple", ""); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(tempDir(t), "bad.maFile")
	if err := os.WriteFile(bad, []byte(`{"shared_secret":"not-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	results, err := service.ImportPlaintext([]string{bad, "relative.maFile"}, "correct horse battery staple", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Imported || results[0].ErrorCode != "invalid_mafile" || results[1].ErrorCode != "path_not_absolute" {
		t.Fatalf("results = %#v", results)
	}
	for _, result := range results {
		if strings.Contains(result.ErrorCode, "secret") {
			t.Fatalf("secret-shaped error leaked: %#v", result)
		}
	}
}

func TestInitializeRequiresAndConfiguresAppPasswordRecovery(t *testing.T) {
	root := useSettingsRoot(t)
	service := NewService()
	t.Cleanup(func() {
		security.SetSteamGuardLifecycleHook(nil)
		_ = security.Lock()
	})
	const appPassword = "separate app password"
	const steamGuardPassword = "separate Steam Guard password"
	if err := security.SetAppPassword(appPassword); err != nil {
		t.Fatal(err)
	}
	if err := security.EnableSavedAccountEncryption(appPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Initialize(steamGuardPassword, "wrong app password"); !errors.Is(err, security.ErrInvalidPassword) {
		t.Fatalf("wrong app password error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "SteamGuard")); !os.IsNotExist(err) {
		t.Fatalf("wrong app password created vault: %v", err)
	}
	if _, err := service.Initialize(steamGuardPassword, appPassword); err != nil {
		t.Fatal(err)
	}
	if service.vault == nil || !service.vault.HasRecoveryWrapper() {
		t.Fatal("double-encrypted vault has no recovery wrapper")
	}
	if err := service.LockNow(); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.UnlockWithRecovery(steamGuardPassword, appPassword, vault.FixedLease); err != nil {
		t.Fatalf("recovery unlock failed: %v", err)
	}
}

func TestInitializeRejectsReusedAppPassword(t *testing.T) {
	root := useSettingsRoot(t)
	service := NewService()
	t.Cleanup(func() {
		security.SetSteamGuardLifecycleHook(nil)
		_ = security.Lock()
	})
	const password = "one password must not protect both layers"
	if err := security.SetAppPassword(password); err != nil {
		t.Fatal(err)
	}
	if err := security.EnableSavedAccountEncryption(password); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Initialize(password, password); !errors.Is(err, ErrPasswordReuse) {
		t.Fatalf("reused password error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "SteamGuard")); !os.IsNotExist(err) {
		t.Fatalf("reused password created vault: %v", err)
	}
}

func TestChangePasswordRequiresAppPasswordAndInvalidatesBackup(t *testing.T) {
	useSettingsRoot(t)
	service := NewService()
	t.Cleanup(func() {
		security.SetSteamGuardLifecycleHook(nil)
		_ = security.Lock()
	})
	const appPassword = "separate app password"
	const currentPassword = "current Steam Guard password"
	const nextPassword = "next Steam Guard password"
	if err := security.SetAppPassword(appPassword); err != nil {
		t.Fatal(err)
	}
	if err := security.EnableSavedAccountEncryption(appPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Initialize(currentPassword, appPassword); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.LastVerifiedBackup = "2026-07-22T12:00:00Z"
	settings.LastVerifiedBackupPath = `C:\backups\SteamGuard-verified`
	if err := SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	if err := service.ChangePassword(currentPassword, nextPassword, "wrong app password"); !errors.Is(err, security.ErrInvalidPassword) {
		t.Fatalf("wrong app password error = %v", err)
	}
	if err := service.ChangePassword(currentPassword, appPassword, appPassword); !errors.Is(err, ErrPasswordReuse) {
		t.Fatalf("reused password error = %v", err)
	}
	if err := service.ChangePassword(currentPassword, nextPassword, appPassword); err != nil {
		t.Fatal(err)
	}
	settings, err = LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.LastVerifiedBackup != "" || settings.LastVerifiedBackupPath != "" {
		t.Fatalf("password change retained stale backup status: %#v", settings)
	}
	if err := service.vault.UnlockWithRecovery(currentPassword, appPassword, vault.FixedLease); !errors.Is(err, vault.ErrInvalidPassword) {
		t.Fatalf("old Steam Guard password error = %v", err)
	}
	if err := service.vault.UnlockWithRecovery(nextPassword, appPassword, vault.FixedLease); err != nil {
		t.Fatalf("new Steam Guard password failed: %v", err)
	}
}

func TestChangeSteamGuardPasswordDoesNotRequireAppPasswordWithoutOuterEncryption(t *testing.T) {
	useSettingsRoot(t)
	service := NewService()
	t.Cleanup(func() {
		security.SetSteamGuardLifecycleHook(nil)
		_ = security.Lock()
	})
	const appPassword = "app access password only"
	const currentPassword = "current Steam Guard password"
	const nextPassword = "next Steam Guard password"
	if err := security.SetAppPassword(appPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Initialize(currentPassword, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.ChangePassword(currentPassword, nextPassword, ""); err != nil {
		t.Fatal(err)
	}
	if err := service.vault.Unlock(nextPassword, vault.FixedLease); err != nil {
		t.Fatalf("new independent Steam Guard password failed: %v", err)
	}
}

func TestSensitiveViewContentProtectionLeases(t *testing.T) {
	service := newServiceForTest()
	var protectionStates []bool
	service.setMainContentProtectionFn = func(enabled bool) error {
		protectionStates = append(protectionStates, enabled)
		return nil
	}

	first := issueSensitiveGrant(t, service, "76561198000000000", "request-content-0001")
	second := issueSensitiveGrant(t, service, "76561198000000001", "request-content-0002")
	if first.Lease == second.Lease || first.Capability == second.Capability {
		t.Fatalf("duplicate grants %#v and %#v", first, second)
	}
	if err := service.EndSensitiveView("unknown", first.Lease); !errors.Is(err, ErrSensitiveLease) {
		t.Fatalf("unknown lease error = %v", err)
	}
	if err := service.EndSensitiveView(first.Capability, first.Lease); err != nil {
		t.Fatal(err)
	}
	if len(protectionStates) != 1 || !protectionStates[0] {
		t.Fatalf("protection states after first release = %v", protectionStates)
	}
	if err := service.EndSensitiveView(second.Capability, second.Lease); err != nil {
		t.Fatal(err)
	}
	if len(protectionStates) != 2 || protectionStates[1] {
		t.Fatalf("protection states after final release = %v", protectionStates)
	}
}

func TestSensitiveViewKeepsLeaseWhenDisableFails(t *testing.T) {
	service := newServiceForTest()
	disableFails := true
	service.setMainContentProtectionFn = func(enabled bool) error {
		if !enabled && disableFails {
			return errors.New("disable failed")
		}
		return nil
	}
	grant := issueSensitiveGrant(t, service, "76561198000000000", "request-disable-0001")
	if err := service.EndSensitiveView(grant.Capability, grant.Lease); !errors.Is(err, ErrSensitiveView) {
		t.Fatalf("disable error = %v", err)
	}
	disableFails = false
	if err := service.EndSensitiveView(grant.Capability, grant.Lease); err != nil {
		t.Fatal(err)
	}
}

func TestSensitiveViewFailsClosedWhenEnableIsUnavailable(t *testing.T) {
	service := newServiceForTest()
	service.setMainContentProtectionFn = nil
	if err := service.RequestSensitiveView("76561198000000000", "request-unavailable-0001"); !errors.Is(err, ErrSensitiveView) {
		t.Fatalf("nil setter result = %v", err)
	}

	service.setMainContentProtectionFn = func(bool) error { return errors.New("window unavailable") }
	if err := service.RequestSensitiveView("76561198000000000", "request-unavailable-0002"); !errors.Is(err, ErrSensitiveView) {
		t.Fatalf("failed enable result = %v", err)
	}
	if len(service.contentProtectionLeases) != 0 {
		t.Fatalf("failed enable retained leases: %v", service.contentProtectionLeases)
	}
}

func TestServiceImportsPasswordlessLegacySDAFileWithAdjacentManifest(t *testing.T) {
	useSettingsRoot(t)
	service := newServiceForTest()
	service.setMainContentProtectionFn = func(bool) error { return nil }
	const vaultPassword = "correct horse battery staple"
	const legacyPassword = ""
	if _, err := service.Initialize(vaultPassword, ""); err != nil {
		t.Fatal(err)
	}
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x32}, 20)),
		DeviceID:       "android:11234567-89ab-cdef-0123-456789abcdef",
		AccountName:    "legacy_account",
		FullyEnrolled:  true,
		Session:        &mafile.SessionData{SteamID: 76561198000000001},
	}
	plain, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	salt := []byte("12345678")
	iv := []byte("1234567890abcdef")
	sealed := legacyEncryptForServiceTest(t, plain, legacyPassword, salt, iv)
	dir := tempDir(t)
	filename := "76561198000000001.maFile"
	source := filepath.Join(dir, filename)
	if err := os.WriteFile(source, sealed, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(map[string]any{
		"encrypted": true,
		"entries": []map[string]any{{
			"filename": filename, "steamid": uint64(76561198000000001),
			"encryption_salt": base64.StdEncoding.EncodeToString(salt),
			"encryption_iv":   base64.StdEncoding.EncodeToString(iv),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}

	results, err := service.ImportMaFiles([]string{source}, vaultPassword, legacyPassword, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Imported || results[0].SteamID64 != "76561198000000001" {
		t.Fatalf("results = %#v", results)
	}
	grant := issueSensitiveGrant(t, service, "76561198000000001", "request-legacy-0001")
	view, err := service.GetCode("76561198000000001", grant.Capability)
	if err != nil || view == nil || len(view.Code) != 5 {
		t.Fatalf("code = %#v, err = %v", view, err)
	}
}

func issueSensitiveGrant(t *testing.T, service *Service, accountID, requestID string) SensitiveViewGrant {
	t.Helper()
	var grant SensitiveViewGrant
	service.emitMainWindowEventFn = func(name string, data any) error {
		if name != SensitiveViewGrantEvent {
			return nil
		}
		value, ok := data.(SensitiveViewGrant)
		if !ok {
			t.Fatalf("grant event data = %#v", data)
		}
		grant = value
		return nil
	}
	if err := service.RequestSensitiveView(accountID, requestID); err != nil {
		t.Fatal(err)
	}
	if grant.Capability == "" || grant.Lease == "" || grant.AccountID != accountID || grant.RequestID != requestID {
		t.Fatalf("invalid grant = %#v", grant)
	}
	return grant
}

func legacyEncryptForServiceTest(t *testing.T, plaintext []byte, password string, salt, iv []byte) []byte {
	t.Helper()
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append(append([]byte{}, plaintext...), bytes.Repeat([]byte{byte(padding)}, padding)...)
	key := pbkdf2.Key([]byte(password), salt, 50000, 32, sha1.New)
	defer wipe(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(sealed, padded)
	return []byte(base64.StdEncoding.EncodeToString(sealed))
}
