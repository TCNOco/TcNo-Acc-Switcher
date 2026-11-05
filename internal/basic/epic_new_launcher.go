package basic

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/securefile"
	"TcNo-Acc-Switcher/internal/winutil"
)

const (
	credentialSessionVersion         = 1
	credentialSessionMaxFiles        = 128
	credentialSessionMaxFileSize     = 4 << 20
	credentialSessionMaxTotalSize    = 16 << 20
	credentialSessionMaxSnapshotSize = 24 << 20
)

type credentialSessionConfig struct {
	directory      string
	target         string
	snapshotFile   string
	idFile         string
	idPath         string
	jsonFiles      []string
	recoveryCopies int
	platformKey    string
	flow           FlowContext
}

type credentialSessionCredential struct {
	TargetName  string `json:"targetName"`
	UserName    string `json:"userName,omitempty"`
	TargetAlias string `json:"targetAlias,omitempty"`
	Comment     string `json:"comment,omitempty"`
	Persist     uint32 `json:"persist"`
	Blob        []byte `json:"blob"`
}

type credentialSessionSnapshot struct {
	Version    int                         `json:"version"`
	UniqueID   string                      `json:"uniqueId"`
	Credential credentialSessionCredential `json:"credential"`
	AuthFiles  map[string][]byte           `json:"authFiles"`
}

type credentialSessionRecovery struct {
	Version     int                  `json:"version"`
	Credential  *winutil.Credential  `json:"credential,omitempty"`
	Credentials []winutil.Credential `json:"credentials,omitempty"`
	AuthFiles   map[string][]byte    `json:"authFiles"`
	ResetPaths  []sessionPathBackup  `json:"resetPaths,omitempty"`
	Incomplete  bool                 `json:"incomplete,omitempty"`
}

func isCredentialSessionFlow(fc FlowContext) bool {
	return fc.Descriptor.Extras.CredentialSession != nil
}

func credentialSessionConfiguration(fc FlowContext) (credentialSessionConfig, error) {
	var cfg credentialSessionConfig
	settings := fc.Descriptor.Extras.CredentialSession
	if settings == nil || strings.TrimSpace(settings.Target) == "" {
		return cfg, errors.New("credential session target is not configured")
	}
	if len(fc.Descriptor.LoginFiles) != 1 || !strings.EqualFold(fc.Descriptor.UniqueIdMethod, "DPAPI_JSON_SELECT") {
		return cfg, errors.New("credential sessions require one login directory and DPAPI_JSON_SELECT identity")
	}
	for raw := range fc.Descriptor.LoginFiles {
		if isREG(raw) || isJSONSelect(raw) || strings.ContainsAny(raw, "*?") {
			return cfg, errors.New("credential session login path must be an exact directory")
		}
		resolved, err := platform.ResolveSafeDeletePattern(raw, fc.PathCtx)
		if err != nil {
			return cfg, err
		}
		cfg.directory = resolved
	}
	idFile, idPath, ok := parseJSONSelectPlain("JSON_SELECT", fc.Descriptor.UniqueIdFile)
	if !ok || strings.TrimSpace(idPath) == "" {
		return cfg, errors.New("invalid credential session identity selector")
	}
	idFile = expandPlatformPath(idFile, fc.Folder, fc.PathCtx)
	if !strings.EqualFold(filepath.Dir(idFile), cfg.directory) {
		return cfg, errors.New("credential session identity file must be inside the login directory")
	}
	if !validCredentialSessionFileName(settings.SnapshotFile) {
		return cfg, errors.New("invalid credential session snapshot filename")
	}
	for _, name := range settings.JSONFiles {
		if !validCredentialSessionFileName(name) {
			return cfg, errors.New("invalid required JSON filename")
		}
	}
	cfg.target = strings.TrimSpace(settings.Target)
	cfg.snapshotFile = settings.SnapshotFile
	cfg.idFile = filepath.Base(idFile)
	cfg.idPath = idPath
	cfg.jsonFiles = append([]string(nil), settings.JSONFiles...)
	cfg.recoveryCopies = settings.RecoveryCopies
	if cfg.recoveryCopies <= 0 {
		cfg.recoveryCopies = 3
	}
	if cfg.recoveryCopies > 20 {
		return cfg, errors.New("too many credential session recovery copies")
	}
	cfg.platformKey = fc.PlatformKey
	cfg.flow = fc
	return cfg, nil
}

func validCredentialSessionFileName(name string) bool {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name ||
		strings.ContainsAny(name, `/\:<>"|?*`) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	units := 0
	for _, char := range name {
		if unicode.IsControl(char) {
			return false
		}
		units += utf16.RuneLen(char)
		if units > 255 {
			return false
		}
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
		return false
	}
	return true
}

func validateCredentialSessionPath(path string) error {
	if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) {
		return errors.New("session path must be absolute")
	}
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		reparse, err := securefile.IsReparsePoint(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && reparse {
			return fmt.Errorf("session path contains a link: %s", current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return nil
}

func readCredentialSessionFile(path string, limit int64) ([]byte, error) {
	if err := validateCredentialSessionPath(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("session file is not regular or exceeds its size limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		clear(data)
		return nil, err
	}
	if int64(len(data)) > limit {
		clear(data)
		return nil, errors.New("session file exceeds its size limit")
	}
	return data, nil
}

func readCredentialSessionFiles(cfg credentialSessionConfig, allowMissing bool) (map[string][]byte, error) {
	if err := validateCredentialSessionPath(cfg.directory); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(cfg.directory)
	if allowMissing && os.IsNotExist(err) {
		return map[string][]byte{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read session directory: %w", err)
	}
	if len(entries) > credentialSessionMaxFiles {
		return nil, errors.New("session directory has too many entries")
	}
	files := make(map[string][]byte, len(entries))
	total := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !validCredentialSessionFileName(entry.Name()) {
			wipeCredentialSessionFiles(files)
			return nil, errors.New("session directory contains an unsupported entry")
		}
		data, err := readCredentialSessionFile(filepath.Join(cfg.directory, entry.Name()), credentialSessionMaxFileSize)
		if err != nil {
			wipeCredentialSessionFiles(files)
			return nil, err
		}
		files[entry.Name()] = data
		total += len(data)
		if total > credentialSessionMaxTotalSize {
			wipeCredentialSessionFiles(files)
			return nil, errors.New("session data exceeds its size limit")
		}
	}
	return files, nil
}

func validateCredentialSessionFiles(cfg credentialSessionConfig, files map[string][]byte, requireIdentity bool) error {
	if len(files) > credentialSessionMaxFiles {
		return errors.New("too many session files")
	}
	total := 0
	names := map[string]bool{}
	for name, data := range files {
		folded := strings.ToLower(name)
		if !validCredentialSessionFileName(name) || names[folded] {
			return errors.New("invalid or duplicate session filename")
		}
		names[folded] = true
		if len(data) > credentialSessionMaxFileSize {
			return errors.New("session file exceeds its size limit")
		}
		total += len(data)
		if total > credentialSessionMaxTotalSize {
			return errors.New("session data exceeds its size limit")
		}
	}
	if !requireIdentity {
		return nil
	}
	if len(files[cfg.idFile]) == 0 {
		return errors.New("session identity file is missing")
	}
	for _, name := range cfg.jsonFiles {
		data, ok := files[name]
		if !ok || !json.Valid(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})) {
			return fmt.Errorf("session JSON file %q is missing or invalid", name)
		}
	}
	return nil
}

func credentialSessionAccountID(cfg credentialSessionConfig, files map[string][]byte) (string, error) {
	if err := validateCredentialSessionFiles(cfg, files, true); err != nil {
		return "", err
	}
	plain, err := winutil.DPAPIUnprotect(files[cfg.idFile])
	if err != nil {
		return "", fmt.Errorf("decrypt session identity: %w", err)
	}
	defer clear(plain)
	id, err := selectDescriptorJSON(plain, cfg.idPath)
	if err != nil {
		return "", err
	}
	return validateCredentialSessionAccountID(id)
}

func validateCredentialSessionAccountID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 128 {
		return "", errors.New("session account ID is empty or too long")
	}
	for _, char := range id {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return "", errors.New("session account ID contains invalid characters")
		}
	}
	return id, nil
}

func snapshotCredential(credential winutil.Credential) credentialSessionCredential {
	return credentialSessionCredential{credential.TargetName, credential.UserName, credential.TargetAlias, credential.Comment, credential.Persist, append([]byte(nil), credential.Blob...)}
}

func (snapshot credentialSessionCredential) native() winutil.Credential {
	return winutil.Credential{TargetName: snapshot.TargetName, UserName: snapshot.UserName, TargetAlias: snapshot.TargetAlias, Comment: snapshot.Comment, Persist: snapshot.Persist, Blob: append([]byte(nil), snapshot.Blob...)}
}

func captureCredentialSession(cfg credentialSessionConfig) (*credentialSessionSnapshot, error) {
	credential, err := winutil.ReadCredential(cfg.target)
	if err != nil {
		return nil, fmt.Errorf("read launcher credential: %w", err)
	}
	defer clear(credential.Blob)
	if err := validateSessionCredential(cfg.target, &credential); err != nil {
		return nil, err
	}
	files, err := readCredentialSessionFiles(cfg, false)
	if err != nil {
		return nil, err
	}
	id, err := credentialSessionAccountID(cfg, files)
	if err != nil {
		wipeCredentialSessionFiles(files)
		return nil, err
	}
	return &credentialSessionSnapshot{credentialSessionVersion, id, snapshotCredential(credential), files}, nil
}

func validateSessionCredential(target string, credential *winutil.Credential) error {
	if credential == nil {
		return nil
	}
	if !winutil.CredentialTargetMatches(target, credential.TargetName) || len(credential.Blob) == 0 || len(credential.Blob) > 2560 || credential.Persist < 1 || credential.Persist > 3 {
		return errors.New("session credential target, size or persistence is invalid")
	}
	return nil
}

func writeProtectedSession(path string, value any) error {
	plain, err := json.Marshal(value)
	if err != nil {
		return err
	}
	defer clear(plain)
	protected, err := winutil.DPAPIProtect(plain)
	if err != nil {
		return err
	}
	defer clear(protected)
	if err := validateCredentialSessionPath(path); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, protected, 0o600)
}

func saveCredentialSessionSnapshot(destRoot, expectedID string, fc FlowContext) error {
	cfg, err := credentialSessionConfiguration(fc)
	if err != nil {
		return err
	}
	snapshot, err := captureCredentialSession(cfg)
	if err != nil {
		return err
	}
	defer wipeCredentialSessionSnapshot(snapshot)
	if !strings.EqualFold(snapshot.UniqueID, strings.TrimSpace(expectedID)) {
		return errors.New("launcher account changed while saving; try again")
	}
	return writeProtectedSession(filepath.Join(destRoot, cfg.snapshotFile), snapshot)
}

func prepareCredentialSessionRestore(fc FlowContext, accountName, expectedID string) (func() error, func(), error) {
	cfg, err := credentialSessionConfiguration(fc)
	if err != nil {
		return nil, nil, err
	}
	source, err := accountCacheDir(fc.PlatformKey, accountName)
	if err != nil {
		return nil, nil, err
	}
	source, cleanup, err := security.AccountRestoreDir(fc.PlatformKey, expectedID, accountName, source)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()
	protected, err := readCredentialSessionFile(filepath.Join(source, cfg.snapshotFile), credentialSessionMaxSnapshotSize)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("saved account %q has no session for this launcher; sign in to that account in the launcher, then use Save current before switching to it", accountName)
		}
		return nil, nil, fmt.Errorf("read saved session for account %q: %w", accountName, err)
	}
	plain, err := winutil.DPAPIUnprotect(protected)
	clear(protected)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt saved session: %w", err)
	}
	defer clear(plain)
	snapshot := &credentialSessionSnapshot{}
	if err := json.Unmarshal(plain, snapshot); err != nil {
		wipeCredentialSessionSnapshot(snapshot)
		return nil, nil, err
	}
	reject := func(err error) (func() error, func(), error) {
		wipeCredentialSessionSnapshot(snapshot)
		return nil, nil, err
	}
	if err := validateCredentialSessionSnapshot(cfg, snapshot, expectedID, credentialSessionAccountID); err != nil {
		return reject(err)
	}
	apply := func() error {
		credential := snapshot.Credential.native()
		defer clear(credential.Blob)
		return changeCredentialSession(cfg, &credential, snapshot.AuthFiles)
	}
	return apply, func() { wipeCredentialSessionSnapshot(snapshot) }, nil
}

func validateCredentialSessionSnapshot(cfg credentialSessionConfig, snapshot *credentialSessionSnapshot, expectedID string, readID func(credentialSessionConfig, map[string][]byte) (string, error)) error {
	if snapshot == nil || strings.TrimSpace(expectedID) == "" || snapshot.Version != credentialSessionVersion || !strings.EqualFold(snapshot.UniqueID, strings.TrimSpace(expectedID)) {
		return errors.New("saved session does not match the selected account or supported version")
	}
	credential := snapshot.Credential.native()
	defer clear(credential.Blob)
	if err := validateSessionCredential(cfg.target, &credential); err != nil {
		return err
	}
	if err := validateCredentialSessionFiles(cfg, snapshot.AuthFiles, true); err != nil {
		return err
	}
	id, err := readID(cfg, snapshot.AuthFiles)
	if err != nil {
		return err
	}
	if !strings.EqualFold(id, strings.TrimSpace(expectedID)) {
		return errors.New("saved token belongs to a different account")
	}
	return nil
}

func changeCredentialSession(cfg credentialSessionConfig, credential *winutil.Credential, files map[string][]byte) error {
	var credentials []winutil.Credential
	if credential != nil {
		credentials = []winutil.Credential{*credential}
	}
	return changeCredentialSessionCredentials(cfg, credentials, files)
}

func validateSessionCredentials(target string, credentials []winutil.Credential) error {
	if len(credentials) > 2 {
		return errors.New("too many credential session targets")
	}
	seen := map[string]bool{}
	for i := range credentials {
		if err := validateSessionCredential(target, &credentials[i]); err != nil {
			return err
		}
		name := strings.ToLower(strings.TrimSpace(credentials[i].TargetName))
		if seen[name] {
			return errors.New("duplicate session credential target")
		}
		seen[name] = true
	}
	return nil
}

func changeCredentialSessionCredentials(cfg credentialSessionConfig, credentials []winutil.Credential, files map[string][]byte) error {
	return changeCredentialSessionState(cfg, credentials, files, nil)
}

func applyCredentialSessionRecovery(cfg credentialSessionConfig, recovery *credentialSessionRecovery) error {
	return changeCredentialSessionState(cfg, recovery.Credentials, recovery.AuthFiles, recovery)
}

func changeCredentialSessionState(cfg credentialSessionConfig, credentials []winutil.Credential, files map[string][]byte, restoring *credentialSessionRecovery) error {
	if err := validateSessionCredentials(cfg.target, credentials); err != nil {
		return err
	}
	if err := validateCredentialSessionFiles(cfg, files, false); err != nil {
		return err
	}
	previousFiles, err := readCredentialSessionFiles(cfg, true)
	if err != nil {
		return err
	}
	defer wipeCredentialSessionFiles(previousFiles)
	previous, err := winutil.ReadCredentials(cfg.target)
	if err != nil && !errors.Is(err, winutil.ErrCredentialNotFound) {
		return err
	}
	defer wipeSessionCredentials(previous)
	if err := validateSessionCredentials(cfg.target, previous); err != nil {
		return err
	}
	resetPaths, err := prepareSessionPaths(cfg.flow, []string{cfg.directory})
	if err != nil {
		return err
	}
	recovery := credentialSessionRecovery{Version: credentialSessionVersion, Credentials: previous, AuthFiles: previousFiles, ResetPaths: resetPaths, Incomplete: true}
	recoveryPath, err := saveCredentialSessionRecovery(cfg, &recovery)
	if err != nil {
		_ = discardSessionPaths(resetPaths)
		return err
	}
	var rollbackErr error
	var restoredPaths []sessionPathBackup
	rollback := func() error {
		var resetErr error
		if len(restoredPaths) > 0 {
			resetErr = revertRestoredSessionPaths(restoredPaths)
		}
		restoreErr := installCredentialSessionCredentials(cfg, previous, previousFiles)
		pathsErr := restoreSessionPaths(resetPaths)
		rollbackErr = errors.Join(resetErr, restoreErr, pathsErr)
		return rollbackErr
	}
	install := func() error {
		if err := installCredentialSessionCredentials(cfg, credentials, files); err != nil {
			return err
		}
		if restoring != nil {
			var err error
			restoredPaths, err = restoreSessionPathsTracked(restoring.ResetPaths)
			return err
		}
		return nil
	}
	err = runSessionChange(func() error { return moveSessionPaths(resetPaths) }, install, rollback)
	if err != nil {
		if rollbackErr == nil {
			recovery.Incomplete = false
			if updateErr := writeProtectedSession(recoveryPath, &recovery); updateErr != nil {
				err = errors.Join(err, updateErr)
			}
			if discardErr := discardSessionPaths(resetPaths); discardErr != nil {
				err = errors.Join(err, discardErr)
			}
		}
		return fmt.Errorf("%w; encrypted recovery saved at %s", err, recoveryPath)
	}
	recovery.Incomplete = false
	if err := writeProtectedSession(recoveryPath, &recovery); err != nil {
		logFlow().Warn("session switched but recovery record could not be finalized", "platform", cfg.platformKey, "err", err)
	}
	if restoring == nil {
		if err := pruneCredentialSessionRecovery(cfg); err != nil {
			logFlow().Warn("session recovery retention failed", "platform", cfg.platformKey, "err", err)
		}
	}
	return nil
}

func runSessionChange(reset, install, rollback func() error) error {
	if err := reset(); err != nil {
		return errors.Join(err, rollback())
	}
	if err := install(); err != nil {
		return errors.Join(err, rollback())
	}
	return nil
}

func installCredentialSessionCredentials(cfg credentialSessionConfig, credentials []winutil.Credential, files map[string][]byte) error {
	if err := validateCredentialSessionFiles(cfg, files, false); err != nil {
		return err
	}
	if err := validateSessionCredentials(cfg.target, credentials); err != nil {
		return err
	}
	if err := validateCredentialSessionPath(cfg.directory); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.directory, 0o700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(cfg.directory), ".tcno-session-files-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(stage, name), files[name], 0o600); err != nil {
			return err
		}
	}
	current, err := os.ReadDir(cfg.directory)
	if err != nil {
		return err
	}
	for _, entry := range current {
		path := filepath.Join(cfg.directory, entry.Name())
		if !entry.Type().IsRegular() || !validCredentialSessionFileName(entry.Name()) {
			return errors.New("session directory contains an unsupported entry")
		}
		if err := validateCredentialSessionPath(path); err != nil {
			return err
		}
	}
	for _, entry := range current {
		if err := os.Remove(filepath.Join(cfg.directory, entry.Name())); err != nil {
			return err
		}
	}
	for _, name := range names {
		if err := os.Rename(filepath.Join(stage, name), filepath.Join(cfg.directory, name)); err != nil {
			return err
		}
	}
	err = winutil.DeleteCredential(cfg.target)
	if err != nil && !errors.Is(err, winutil.ErrCredentialNotFound) {
		return err
	}
	for _, credential := range credentials {
		if err := winutil.WriteCredential(credential); err != nil {
			return err
		}
	}
	return nil
}

func clearCredentialSession(fc FlowContext) error {
	cfg, err := credentialSessionConfiguration(fc)
	if err != nil {
		return err
	}
	return changeCredentialSession(cfg, nil, map[string][]byte{})
}

func credentialSessionRecoveryDirectory(cfg credentialSessionConfig) (string, error) {
	directory, err := platform.CredentialSessionBackupDirectory(cfg.platformKey)
	if err != nil {
		return "", err
	}
	if err := validateCredentialSessionPath(directory); err != nil {
		return "", err
	}
	return directory, nil
}

func saveCredentialSessionRecovery(cfg credentialSessionConfig, recovery *credentialSessionRecovery) (string, error) {
	directory, err := credentialSessionRecoveryDirectory(cfg)
	if err != nil {
		return "", err
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	name := time.Now().UTC().Format("20060102T150405.000000000") + "-" + hex.EncodeToString(random[:]) + ".dpapi"
	path := filepath.Join(directory, name)
	if err := writeProtectedSession(path, recovery); err != nil {
		return "", err
	}
	return path, nil
}

func readCredentialSessionRecovery(path string, cfg credentialSessionConfig) (*credentialSessionRecovery, error) {
	protected, err := readCredentialSessionFile(path, credentialSessionMaxSnapshotSize)
	if err != nil {
		return nil, err
	}
	defer clear(protected)
	plain, err := winutil.DPAPIUnprotect(protected)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	recovery := &credentialSessionRecovery{}
	if err := json.Unmarshal(plain, recovery); err != nil {
		wipeCredentialSessionRecovery(recovery)
		return nil, err
	}
	if recovery.Version != credentialSessionVersion {
		wipeCredentialSessionRecovery(recovery)
		return nil, errors.New("unsupported session recovery version")
	}
	if len(recovery.Credentials) == 0 && recovery.Credential != nil {
		recovery.Credentials = []winutil.Credential{*recovery.Credential}
	}
	if err := validateSessionCredentials(cfg.target, recovery.Credentials); err != nil {
		wipeCredentialSessionRecovery(recovery)
		return nil, err
	}
	if err := validateCredentialSessionFiles(cfg, recovery.AuthFiles, false); err != nil {
		wipeCredentialSessionRecovery(recovery)
		return nil, err
	}
	if err := validateConfiguredSessionBackups(cfg.flow, []string{cfg.directory}, recovery.ResetPaths); err != nil {
		wipeCredentialSessionRecovery(recovery)
		return nil, err
	}
	return recovery, nil
}

func pruneCredentialSessionRecovery(cfg credentialSessionConfig) error {
	directory, err := credentialSessionRecoveryDirectory(cfg)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".dpapi") {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	var errs []error
	kept := 0
	for _, name := range names {
		path := filepath.Join(directory, name)
		recovery, err := readCredentialSessionRecovery(path, cfg)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if recovery.Incomplete || kept < cfg.recoveryCopies {
			if !recovery.Incomplete {
				kept++
			}
			wipeCredentialSessionRecovery(recovery)
			continue
		}
		err = discardSessionPaths(recovery.ResetPaths)
		wipeCredentialSessionRecovery(recovery)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := os.Remove(path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func wipeCredentialSessionFiles(files map[string][]byte) {
	for _, data := range files {
		clear(data)
	}
}
func wipeCredentialSessionSnapshot(snapshot *credentialSessionSnapshot) {
	clear(snapshot.Credential.Blob)
	wipeCredentialSessionFiles(snapshot.AuthFiles)
}
func wipeCredentialSessionRecovery(recovery *credentialSessionRecovery) {
	if recovery.Credential != nil {
		clear(recovery.Credential.Blob)
	}
	wipeSessionCredentials(recovery.Credentials)
	wipeCredentialSessionFiles(recovery.AuthFiles)
}

func wipeSessionCredentials(credentials []winutil.Credential) {
	for i := range credentials {
		clear(credentials[i].Blob)
	}
}
