package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

type quarantineFailure struct {
	Session savedSession
	Err     error
}

type quarantineRecovery struct {
	Version                   int       `json:"version"`
	KDF                       KDFParams `json:"kdf"`
	Salt                      string    `json:"salt"`
	WrappedVaultKeyNonce      string    `json:"wrappedVaultKeyNonce"`
	WrappedVaultKeyCiphertext string    `json:"wrappedVaultKeyCiphertext"`
	CreatedAt                 string    `json:"createdAt"`
}

type QuarantineInfo struct {
	ID        string   `json:"id"`
	CreatedAt string   `json:"createdAt"`
	Accounts  []string `json:"accounts"`
	LogPath   string   `json:"logPath"`
}

type quarantineReportEntry struct {
	PlatformKey string `json:"platformKey"`
	UniqueID    string `json:"uniqueId"`
	AccountName string `json:"accountName"`
	Blob        string `json:"blob"`
	Error       string `json:"error"`
}

func ListQuarantines() ([]QuarantineInfo, error) {
	root, err := vaultRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, vaultQuarantineDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]QuarantineInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		reportPath := filepath.Join(dir, id, "report.json")
		data, err := os.ReadFile(reportPath)
		if err != nil {
			out = append(out, QuarantineInfo{ID: id, LogPath: reportPath})
			continue
		}
		var report struct {
			CreatedAt string   `json:"createdAt"`
			Accounts  []string `json:"accounts"`
		}
		_ = json.Unmarshal(data, &report)
		out = append(out, QuarantineInfo{ID: id, CreatedAt: report.CreatedAt, Accounts: report.Accounts, LogPath: reportPath})
	}
	return out, nil
}

func DeleteQuarantine(id string) error {
	id = paths.SanitizePathSegment(id)
	if id == "" {
		return fmt.Errorf("invalid quarantine id")
	}
	root, err := vaultRoot()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(root, vaultQuarantineDir, id))
}

func RetryQuarantineImport(id, password string) error {
	id = paths.SanitizePathSegment(id)
	if id == "" {
		return fmt.Errorf("invalid quarantine id")
	}
	root, err := vaultRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, vaultQuarantineDir, id)
	var recovery quarantineRecovery
	if err := readJSON(filepath.Join(dir, "recovery.json"), &recovery); err != nil {
		return err
	}
	if recovery.Version != securityVersion {
		return fmt.Errorf("unsupported quarantine recovery version %d", recovery.Version)
	}
	salt, err := decode(recovery.Salt)
	if err != nil {
		return err
	}
	derived := deriveKey(password, salt, recovery.KDF)
	nonce, err := decode(recovery.WrappedVaultKeyNonce)
	if err != nil {
		return err
	}
	wrapped, err := decode(recovery.WrappedVaultKeyCiphertext)
	if err != nil {
		return err
	}
	master, err := openWithKey(derived, nonce, wrapped, []byte(wrappedKeyAAD))
	if err != nil {
		return ErrInvalidPassword
	}
	var report struct {
		Failures []quarantineReportEntry `json:"failures"`
	}
	if err := readJSON(filepath.Join(dir, "report.json"), &report); err != nil {
		return err
	}
	if len(report.Failures) == 0 {
		return fmt.Errorf("quarantine has no account blobs")
	}
	for _, f := range report.Failures {
		blobPath := filepath.Join(dir, filepath.FromSlash(f.Blob))
		plain, err := decryptAccountBlobFile(master, blobPath, f.PlatformKey, f.UniqueID)
		if err != nil {
			return err
		}
		cacheRoot, err := loginCacheRoot()
		if err != nil {
			return err
		}
		dest := filepath.Join(cacheRoot, paths.SanitizePathSegment(f.PlatformKey), paths.SanitizePathSegment(f.AccountName))
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		if err := unpackDir(plain, dest); err != nil {
			return err
		}
		if err := addAccountToIDs(f.PlatformKey, f.UniqueID, f.AccountName); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	emitStatusChanged()
	return nil
}

func quarantineFailures(password string, masterKey []byte, failures []quarantineFailure) error {
	if len(failures) == 0 {
		return nil
	}
	sf, ok, err := loadSecurityFile()
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasswordNotSet
	}
	root, err := vaultRoot()
	if err != nil {
		return err
	}
	id := time.Now().UTC().Format("20060102T150405.000000000")
	dir := filepath.Join(root, vaultQuarantineDir, id)
	if err := os.MkdirAll(filepath.Join(dir, "accounts"), 0o700); err != nil {
		return err
	}
	accounts := make([]string, 0, len(failures))
	report := struct {
		CreatedAt string                  `json:"createdAt"`
		Accounts  []string                `json:"accounts"`
		Failures  []quarantineReportEntry `json:"failures"`
	}{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, f := range failures {
		accountLabel := f.Session.PlatformKey + "/" + f.Session.UniqueID
		if f.Session.AccountName != "" {
			accountLabel += " (" + f.Session.AccountName + ")"
		}
		accounts = append(accounts, accountLabel)
		dstDir := filepath.Join(dir, "accounts", paths.SanitizePathSegment(f.Session.PlatformKey))
		if err := os.MkdirAll(dstDir, 0o700); err != nil {
			return err
		}
		dst := filepath.Join(dstDir, paths.SanitizePathSegment(f.Session.UniqueID)+accountBlobExt)
		_ = os.Rename(f.Session.BlobPath, dst)
		report.Failures = append(report.Failures, quarantineReportEntry{
			PlatformKey: f.Session.PlatformKey,
			UniqueID:    f.Session.UniqueID,
			AccountName: f.Session.AccountName,
			Blob:        filepath.ToSlash(filepath.Join("accounts", paths.SanitizePathSegment(f.Session.PlatformKey), paths.SanitizePathSegment(f.Session.UniqueID)+accountBlobExt)),
			Error:       f.Err.Error(),
		})
	}
	report.Accounts = accounts
	if err := writeJSON(filepath.Join(dir, "report.json"), report, 0o600); err != nil {
		return err
	}
	recovery := quarantineRecovery{
		Version:                   securityVersion,
		KDF:                       sf.KDF,
		Salt:                      sf.Salt,
		WrappedVaultKeyNonce:      sf.WrappedVaultKeyNonce,
		WrappedVaultKeyCiphertext: sf.WrappedVaultKeyCiphertext,
		CreatedAt:                 time.Now().UTC().Format(time.RFC3339),
	}
	return writeJSON(filepath.Join(dir, "recovery.json"), recovery, 0o600)
}

func writeJSON(path string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomicDurable(path, append(data, '\n'), perm)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func addAccountToIDs(platformKey, uniqueID, accountName string) error {
	root, err := loginCacheRoot()
	if err != nil {
		return err
	}
	p := filepath.Join(root, paths.SanitizePathSegment(platformKey), "ids.json")
	data, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	raw := map[string]json.RawMessage{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &raw)
	}
	var ids map[string]string
	if v := raw["ids"]; len(v) > 0 {
		_ = json.Unmarshal(v, &ids)
	}
	if ids == nil {
		ids = map[string]string{}
	}
	ids[strings.TrimSpace(uniqueID)] = strings.TrimSpace(accountName)
	nextIDs, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	raw["ids"] = nextIDs
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomicDurable(p, append(out, '\n'), 0o644)
}
