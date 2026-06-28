package security

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

const (
	vaultDirName       = "Vault"
	vaultAccountsDir   = "accounts"
	vaultTmpDir        = "tmp"
	vaultStagingDir    = "staging"
	vaultJournalDir    = "journal"
	vaultQuarantineDir = "quarantine"

	accountBlobVersion = 1
	accountBlobExt     = ".tcvault"
)

type encryptedAccountBlob struct {
	Version     int    `json:"version"`
	PlatformKey string `json:"platformKey"`
	UniqueID    string `json:"uniqueId"`
	AccountName string `json:"accountName"`
	Algorithm   string `json:"algorithm"`
	Compression string `json:"compression"`
	Nonce       string `json:"nonce"`
	Ciphertext  string `json:"ciphertext"`
	CreatedAt   string `json:"createdAt"`
}

type idsFileLite struct {
	IDs map[string]string `json:"ids"`
}

type savedSession struct {
	PlatformKey string
	UniqueID    string
	AccountName string
	PlainDir    string
	BlobPath    string
}

type InterruptedRestoreInfo struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"createdAt"`
	PlatformKey string `json:"platformKey"`
	UniqueID    string `json:"uniqueId"`
	AccountName string `json:"accountName"`
	JournalPath string `json:"journalPath"`
}

type AccountSave struct {
	PlatformKey string
	UniqueID    string
	AccountName string
	DestRoot    string
	Encrypted   bool
	journalPath string
	cleanup     func()
}

func BeginAccountSave(platformKey, uniqueID, accountName, normalDir string) (AccountSave, error) {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	accountName = strings.TrimSpace(accountName)
	if !SavedAccountDataEncrypted() {
		return AccountSave{PlatformKey: platformKey, UniqueID: uniqueID, AccountName: accountName, DestRoot: normalDir}, nil
	}
	if err := RequireUnlocked(); err != nil {
		return AccountSave{}, err
	}
	dir, err := newStagingDir("save", platformKey, uniqueID)
	if err != nil {
		return AccountSave{}, err
	}
	journal, err := writeJournal("save", map[string]any{
		"platformKey": platformKey,
		"uniqueId":    uniqueID,
		"accountName": accountName,
	})
	if err != nil {
		_ = os.RemoveAll(dir)
		return AccountSave{}, err
	}
	return AccountSave{
		PlatformKey: platformKey,
		UniqueID:    uniqueID,
		AccountName: accountName,
		DestRoot:    dir,
		Encrypted:   true,
		journalPath: journal,
		cleanup: func() {
			_ = os.RemoveAll(dir)
			_ = os.Remove(journal)
		},
	}, nil
}

func CommitAccountSave(save AccountSave, normalDir string) error {
	if !save.Encrypted {
		return nil
	}
	defer save.cleanup()
	key, err := defaultManager.unlockedMasterKey()
	if err != nil {
		return err
	}
	if err := writeAccountBlob(key, save.PlatformKey, save.UniqueID, save.AccountName, save.DestRoot); err != nil {
		return err
	}
	if strings.TrimSpace(normalDir) != "" {
		_ = fsutil.RemoveAllWithRetry(normalDir, 2*time.Second, os.RemoveAll)
	}
	if save.journalPath != "" {
		_ = os.Remove(save.journalPath)
	}
	return nil
}

func CleanupAccountSave(save AccountSave) {
	if save.cleanup != nil {
		save.cleanup()
	}
}

func AccountRestoreDir(platformKey, uniqueID, accountName, normalDir string) (string, func(), error) {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	accountName = strings.TrimSpace(accountName)
	if !SavedAccountDataEncrypted() {
		return normalDir, func() {}, nil
	}
	key, err := defaultManager.unlockedMasterKey()
	if err != nil {
		return "", nil, err
	}
	dir, err := newStagingDir("restore", platformKey, uniqueID)
	if err != nil {
		return "", nil, err
	}
	journal, err := writeJournal("restore", map[string]any{
		"platformKey": platformKey,
		"uniqueId":    uniqueID,
		"accountName": accountName,
	})
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(dir)
		_ = os.Remove(journal)
	}
	if err := extractAccountBlob(key, platformKey, uniqueID, dir); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}

func RemoveAccountCache(platformKey, uniqueID, accountName, normalDir string) error {
	if SavedAccountDataEncrypted() {
		journal, err := writeJournal("delete", map[string]any{
			"platformKey": strings.TrimSpace(platformKey),
			"uniqueId":    strings.TrimSpace(uniqueID),
			"accountName": strings.TrimSpace(accountName),
		})
		if err != nil {
			return err
		}
		defer os.Remove(journal)
		p, err := accountBlobPath(platformKey, uniqueID)
		if err != nil {
			return err
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if strings.TrimSpace(normalDir) != "" {
		return fsutil.RemoveAllWithRetry(normalDir, 2*time.Second, os.RemoveAll)
	}
	return nil
}

func AccountBlobValid(platformKey, uniqueID string) bool {
	if !SavedAccountDataEncrypted() {
		return true
	}
	key, err := defaultManager.unlockedMasterKey()
	if err != nil {
		return false
	}
	p, err := accountBlobPath(platformKey, uniqueID)
	if err != nil {
		return false
	}
	_, err = decryptAccountBlobFile(key, p, platformKey, uniqueID)
	return err == nil
}

func EnableSavedAccountEncryption(password string) error {
	key, err := unlockWithPassword(password)
	if err != nil {
		return err
	}
	defaultManager.mu.Lock()
	defaultManager.masterKey = key
	defaultManager.mu.Unlock()
	defaultManager.setBusy(true)
	defer defaultManager.setBusy(false)
	if SavedAccountDataEncrypted() {
		return nil
	}
	sessions, err := collectPlainSavedSessions()
	if err != nil {
		return err
	}
	journal, err := writeJournal("enable-encryption", map[string]any{"sessions": len(sessions)})
	if err != nil {
		return err
	}
	var written []string
	for _, s := range sessions {
		if err := writeAccountBlob(key, s.PlatformKey, s.UniqueID, s.AccountName, s.PlainDir); err != nil {
			for _, p := range written {
				_ = os.Remove(p)
			}
			_ = os.Remove(journal)
			return err
		}
		written = append(written, s.BlobPath)
	}
	if err := updateSecurityFile(func(sf *securityFile) error {
		sf.SavedAccountDataEncrypted = true
		return nil
	}); err != nil {
		_ = os.Remove(journal)
		return err
	}
	emitStatusChanged()
	for _, s := range sessions {
		_ = fsutil.RemoveAllWithRetry(s.PlainDir, 2*time.Second, os.RemoveAll)
	}
	_ = os.Remove(journal)
	return nil
}

func DisableSavedAccountEncryption(password string) error {
	key, err := unlockWithPassword(password)
	if err != nil {
		return err
	}
	return disableSavedAccountEncryptionWithKey(password, key, false)
}

func disableSavedAccountEncryptionWithKey(password string, key []byte, removingPassword bool) error {
	defaultManager.setBusy(true)
	defer defaultManager.setBusy(false)
	if !SavedAccountDataEncrypted() {
		return nil
	}
	sessions, err := collectEncryptedSessions()
	if err != nil {
		return err
	}
	journal, err := writeJournal("disable-encryption", map[string]any{"sessions": len(sessions), "removePassword": removingPassword})
	if err != nil {
		return err
	}
	var failures []quarantineFailure
	for _, s := range sessions {
		if err := extractAccountBlob(key, s.PlatformKey, s.UniqueID, s.PlainDir); err != nil {
			failures = append(failures, quarantineFailure{Session: s, Err: err})
			continue
		}
	}
	if len(failures) > 0 {
		if err := quarantineFailures(password, key, failures); err != nil {
			_ = os.Remove(journal)
			return err
		}
		for _, f := range failures {
			_ = removeAccountFromIDs(f.Session.PlatformKey, f.Session.UniqueID)
		}
	}
	if err := updateSecurityFile(func(sf *securityFile) error {
		sf.SavedAccountDataEncrypted = false
		return nil
	}); err != nil {
		_ = os.Remove(journal)
		return err
	}
	emitStatusChanged()
	for _, s := range sessions {
		_ = os.Remove(s.BlobPath)
	}
	_ = os.Remove(journal)
	return nil
}

func collectPlainSavedSessions() ([]savedSession, error) {
	root, err := loginCacheRoot()
	if err != nil {
		return nil, err
	}
	platforms, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []savedSession
	for _, pe := range platforms {
		if !pe.IsDir() || strings.EqualFold(pe.Name(), "Steam") {
			continue
		}
		platformKey := pe.Name()
		ids, err := readIDsLite(filepath.Join(root, platformKey, "ids.json"))
		if err != nil {
			return nil, err
		}
		for uid, accountName := range ids {
			dir := filepath.Join(root, platformKey, paths.SanitizePathSegment(accountName))
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				blob, err := accountBlobPath(platformKey, uid)
				if err != nil {
					return nil, err
				}
				out = append(out, savedSession{PlatformKey: platformKey, UniqueID: uid, AccountName: accountName, PlainDir: dir, BlobPath: blob})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PlatformKey == out[j].PlatformKey {
			return out[i].UniqueID < out[j].UniqueID
		}
		return out[i].PlatformKey < out[j].PlatformKey
	})
	return out, nil
}

func collectEncryptedSessions() ([]savedSession, error) {
	root, err := loginCacheRoot()
	if err != nil {
		return nil, err
	}
	platforms, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []savedSession
	for _, pe := range platforms {
		if !pe.IsDir() || strings.EqualFold(pe.Name(), "Steam") {
			continue
		}
		platformKey := pe.Name()
		ids, err := readIDsLite(filepath.Join(root, platformKey, "ids.json"))
		if err != nil {
			return nil, err
		}
		for uid, accountName := range ids {
			blob, err := accountBlobPath(platformKey, uid)
			if err != nil {
				return nil, err
			}
			if _, err := os.Stat(blob); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			out = append(out, savedSession{
				PlatformKey: platformKey,
				UniqueID:    uid,
				AccountName: accountName,
				PlainDir:    filepath.Join(root, platformKey, paths.SanitizePathSegment(accountName)),
				BlobPath:    blob,
			})
		}
	}
	return out, nil
}

func writeAccountBlob(masterKey []byte, platformKey, uniqueID, accountName, dir string) error {
	plain, err := packDir(dir)
	if err != nil {
		return err
	}
	aad := accountBlobAAD(platformKey, uniqueID)
	nonce, ciphertext, err := sealWithKey(masterKey, plain, aad)
	if err != nil {
		return err
	}
	blob := encryptedAccountBlob{
		Version:     accountBlobVersion,
		PlatformKey: strings.TrimSpace(platformKey),
		UniqueID:    strings.TrimSpace(uniqueID),
		AccountName: strings.TrimSpace(accountName),
		Algorithm:   "AES-256-GCM",
		Compression: "gzip-fastest-tar",
		Nonce:       encode(nonce),
		Ciphertext:  encode(ciphertext),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(blob, "", "  ")
	if err != nil {
		return err
	}
	path, err := accountBlobPath(platformKey, uniqueID)
	if err != nil {
		return err
	}
	if err := writeFileAtomicDurable(path, append(data, '\n'), 0o600); err != nil {
		return err
	}
	testDir, err := os.MkdirTemp(filepath.Dir(path), ".verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(testDir)
	return extractAccountBlob(masterKey, platformKey, uniqueID, testDir)
}

func extractAccountBlob(masterKey []byte, platformKey, uniqueID, dest string) error {
	if strings.TrimSpace(dest) == "" {
		return fmt.Errorf("empty account blob extract destination")
	}
	p, err := accountBlobPath(platformKey, uniqueID)
	if err != nil {
		return err
	}
	plain, err := decryptAccountBlobFile(masterKey, p, platformKey, uniqueID)
	if err != nil {
		return err
	}
	if err := fsutil.RemoveAllWithRetry(dest, 2*time.Second, os.RemoveAll); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return unpackDir(plain, dest)
}

func decryptAccountBlobFile(masterKey []byte, path, platformKey, uniqueID string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var blob encryptedAccountBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, err
	}
	if blob.Version != accountBlobVersion {
		return nil, fmt.Errorf("unsupported account blob version %d", blob.Version)
	}
	if !strings.EqualFold(strings.TrimSpace(blob.PlatformKey), strings.TrimSpace(platformKey)) ||
		!strings.EqualFold(strings.TrimSpace(blob.UniqueID), strings.TrimSpace(uniqueID)) {
		return nil, fmt.Errorf("account blob identity mismatch")
	}
	nonce, err := decode(blob.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := decode(blob.Ciphertext)
	if err != nil {
		return nil, err
	}
	return openWithKey(masterKey, nonce, ciphertext, accountBlobAAD(platformKey, uniqueID))
}

func packDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(gz)
	err = filepath.WalkDir(dir, func(path string, de os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		info, err := de.Info()
		if err != nil {
			return err
		}
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = rel
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if de.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if closeErr := tw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gz.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func unpackDir(data []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanDest := filepath.Clean(dest)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(h.Name))
		if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(filepath.Separator)) || name == ".." {
			return fmt.Errorf("unsafe path in account blob: %s", h.Name)
		}
		target := filepath.Join(cleanDest, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			continue
		}
	}
}

func accountBlobAAD(platformKey, uniqueID string) []byte {
	return []byte("tcno-account-blob-v1\x00" + strings.TrimSpace(platformKey) + "\x00" + strings.TrimSpace(uniqueID))
}

func accountBlobPath(platformKey, uniqueID string) (string, error) {
	root, err := vaultRoot()
	if err != nil {
		return "", err
	}
	p := paths.SanitizePathSegment(platformKey)
	u := paths.SanitizePathSegment(uniqueID)
	if p == "" || u == "" {
		return "", fmt.Errorf("invalid account blob identity")
	}
	return filepath.Join(root, vaultAccountsDir, p, u+accountBlobExt), nil
}

func vaultRoot() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, vaultDirName), nil
}

func loginCacheRoot() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "LoginCache"), nil
}

func newStagingDir(kind, platformKey, uniqueID string) (string, error) {
	root, err := vaultRoot()
	if err != nil {
		return "", err
	}
	base := filepath.Join(root, vaultStagingDir)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	prefix := paths.SanitizePathSegment(kind+"-"+platformKey+"-"+uniqueID) + "-"
	return os.MkdirTemp(base, prefix)
}

func readIDsLite(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var f idsFileLite
	if err := json.Unmarshal(data, &f); err != nil {
		return map[string]string{}, nil
	}
	if f.IDs == nil {
		f.IDs = map[string]string{}
	}
	return f.IDs, nil
}

func writeIDsLite(path string, ids map[string]string) error {
	f := idsFileLite{IDs: ids}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomicDurable(path, append(data, '\n'), 0o644)
}

func removeAccountFromIDs(platformKey, uniqueID string) error {
	root, err := loginCacheRoot()
	if err != nil {
		return err
	}
	p := filepath.Join(root, paths.SanitizePathSegment(platformKey), "ids.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	var ids map[string]string
	if v := raw["ids"]; len(v) > 0 {
		_ = json.Unmarshal(v, &ids)
	}
	if ids == nil {
		ids = map[string]string{}
	}
	delete(ids, uniqueID)
	nextIDs, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	raw["ids"] = nextIDs
	if v := raw["lastused"]; len(v) > 0 {
		var last map[string]string
		if json.Unmarshal(v, &last) == nil {
			delete(last, uniqueID)
			nextLast, err := json.Marshal(last)
			if err != nil {
				return err
			}
			raw["lastused"] = nextLast
		}
	}
	if v := raw["accountTags"]; len(v) > 0 {
		var accountTags map[string][]string
		if json.Unmarshal(v, &accountTags) == nil {
			delete(accountTags, uniqueID)
			nextTags, err := json.Marshal(accountTags)
			if err != nil {
				return err
			}
			raw["accountTags"] = nextTags
		}
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomicDurable(p, append(out, '\n'), 0o644)
}

func removeVaultAccounts() error {
	root, err := vaultRoot()
	if err != nil {
		return err
	}
	return fsutil.RemoveAllWithRetry(filepath.Join(root, vaultAccountsDir), 2*time.Second, os.RemoveAll)
}

func writeJournal(kind string, payload map[string]any) (string, error) {
	root, err := vaultRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, vaultJournalDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	payload["kind"] = kind
	payload["createdAt"] = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, time.Now().UTC().Format("20060102T150405.000000000")+"-"+paths.SanitizePathSegment(kind)+".json")
	return p, writeFileAtomicDurable(p, append(data, '\n'), 0o600)
}

func hasInterruptedRestoreJournal() bool {
	matches, err := interruptedRestoreJournalPaths()
	return err == nil && len(matches) > 0
}

func ListInterruptedRestores() ([]InterruptedRestoreInfo, error) {
	matches, err := interruptedRestoreJournalPaths()
	if err != nil {
		return nil, err
	}
	out := make([]InterruptedRestoreInfo, 0, len(matches))
	for _, p := range matches {
		var entry struct {
			CreatedAt   string `json:"createdAt"`
			PlatformKey string `json:"platformKey"`
			UniqueID    string `json:"uniqueId"`
			AccountName string `json:"accountName"`
		}
		if err := readJSON(p, &entry); err != nil {
			out = append(out, InterruptedRestoreInfo{
				ID:          strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)),
				JournalPath: filepath.ToSlash(filepath.Join(vaultJournalDir, filepath.Base(p))),
			})
			continue
		}
		out = append(out, InterruptedRestoreInfo{
			ID:          strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)),
			CreatedAt:   entry.CreatedAt,
			PlatformKey: entry.PlatformKey,
			UniqueID:    entry.UniqueID,
			AccountName: entry.AccountName,
			JournalPath: filepath.ToSlash(filepath.Join(vaultJournalDir, filepath.Base(p))),
		})
	}
	return out, nil
}

func RepairInterruptedRestore() error {
	matches, err := interruptedRestoreJournalPaths()
	if err != nil {
		return err
	}
	journal, err := writeJournal("repair-interrupted", map[string]any{
		"restoreJournals": len(matches),
	})
	if err != nil {
		return err
	}
	CleanupTransientState()
	for _, p := range matches {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(journal)
			return err
		}
	}
	_ = os.Remove(journal)
	emitStatusChanged()
	return nil
}

func interruptedRestoreJournalPaths() ([]string, error) {
	root, err := vaultRoot()
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(root, vaultJournalDir, "*restore*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

func countQuarantines() int {
	root, err := vaultRoot()
	if err != nil {
		return 0
	}
	entries, err := os.ReadDir(filepath.Join(root, vaultQuarantineDir))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n
}

func CleanupTransientState() {
	root, err := vaultRoot()
	if err != nil {
		return
	}
	_ = fsutil.RemoveAllWithRetry(filepath.Join(root, vaultStagingDir), 2*time.Second, os.RemoveAll)
	_ = fsutil.RemoveAllWithRetry(filepath.Join(root, vaultTmpDir), 2*time.Second, os.RemoveAll)
}
