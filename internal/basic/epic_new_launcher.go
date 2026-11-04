package basic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/winutil"
)

const (
	epicNewLauncherPlatformKey = "Epic Games"
	epicNewLauncherCredentialTarget = "LegacyGeneric:target=credentials.com.epicgames.eglv2.prod"
	epicNewLauncherAccessTokenFile = "access-token-prod.dpapi"
	epicNewLauncherAuthInfoFile = "auth.json"
	epicNewLauncherSnapshotFile = "epic-new-launcher-session.dpapi"

	epicNewLauncherSnapshotVersion = 1
	epicNewLauncherMaxAuthFiles = 128
	epicNewLauncherMaxAuthFileSize = 4 << 20
	epicNewLauncherMaxAuthTotalSize = 16 << 20
)

type epicNewLauncherCredentialSnapshot struct {
	TargetName  string `json:"targetName"`
	UserName    string `json:"userName,omitempty"`
	TargetAlias string `json:"targetAlias,omitempty"`
	Comment     string `json:"comment,omitempty"`
	Persist     uint32 `json:"persist"`
	Blob        []byte `json:"blob"`
}

type epicNewLauncherSnapshot struct {
	Version    int                                  `json:"version"`
	UniqueID   string                               `json:"uniqueId"`
	Credential epicNewLauncherCredentialSnapshot    `json:"credential"`
	AuthFiles  map[string][]byte                   `json:"authFiles"`
}

type epicNewLauncherToken struct {
	AccountID string `json:"account_id"`
}

// isEpicNewLauncherFlow ensures the account flow only uses the new launcher's
// session handling when the selected Epic installation is the per-user build.
func isEpicNewLauncherFlow(fc FlowContext) bool {
	return strings.EqualFold(strings.TrimSpace(fc.PlatformKey), epicNewLauncherPlatformKey) &&
		isEpicNewLauncherFolder(fc.Folder)
}

// The current new launcher is installed under LOCALAPPDATA\Epic Games\Launcher.
// The legacy launcher executable is under Launcher\Portal\Binaries, so it
// remains on the existing registry-and-config path.
func isEpicNewLauncherFolder(folder string) bool {
	folder = strings.TrimSpace(folder)
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if folder == "" || localAppData == "" {
		return false
	}

	root := filepath.Clean(filepath.Join(localAppData, "Epic Games", "Launcher"))
	candidate := filepath.Clean(folder)
	// Some platform path providers hand back the executable rather than its
	// containing directory. Normalize both forms before comparing.
	if strings.EqualFold(filepath.Ext(candidate), ".exe") {
		candidate = filepath.Dir(candidate)
	}
	return strings.EqualFold(candidate, root)
}

func epicNewLauncherAuthDir() (string, error) {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return "", errors.New("LOCALAPPDATA is not set; cannot locate the new Epic Games Launcher session")
	}
	return filepath.Join(localAppData, "com.epicgames.eglv2", "auth"), nil
}

func readEpicNewLauncherAuthFiles() (map[string][]byte, error) {
	authDir, err := epicNewLauncherAuthDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(authDir)
	if err != nil {
		return nil, fmt.Errorf("read new Epic launcher auth folder: %w", err)
	}
	if len(entries) > epicNewLauncherMaxAuthFiles {
		return nil, fmt.Errorf("new Epic launcher auth folder has too many entries")
	}

	files := make(map[string][]byte, len(entries))
	fail := func(err error) (map[string][]byte, error) {
		wipeEpicNewLauncherAuthFiles(files)
		return nil, err
	}
	total := 0
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return fail(fmt.Errorf("inspect new Epic launcher auth file %q: %w", entry.Name(), err))
		}
		// Never follow symlinks or copy directories from the auth folder.
		if !info.Mode().IsRegular() {
			continue
		}
		name := entry.Name()
		if !validEpicNewLauncherAuthFileName(name) {
			return fail(errors.New("unexpected filename in new Epic launcher auth folder"))
		}
		if info.Size() < 0 || info.Size() > epicNewLauncherMaxAuthFileSize {
			return fail(fmt.Errorf("new Epic launcher auth file %q exceeds the size limit", name))
		}
		total += int(info.Size())
		if total > epicNewLauncherMaxAuthTotalSize {
			return fail(errors.New("new Epic launcher auth data exceeds the size limit"))
		}
		data, err := os.ReadFile(filepath.Join(authDir, name))
		if err != nil {
			return fail(fmt.Errorf("read new Epic launcher auth file %q: %w", name, err))
		}
		// Recheck after reading to avoid trusting a stale stat size if the file
		// changed between Info and ReadFile.
		if len(data) > epicNewLauncherMaxAuthFileSize {
			clear(data)
			return fail(fmt.Errorf("new Epic launcher auth file %q exceeds the size limit", name))
		}
		total = total - int(info.Size()) + len(data)
		if total > epicNewLauncherMaxAuthTotalSize {
			clear(data)
			return fail(errors.New("new Epic launcher auth data exceeds the size limit"))
		}
		files[name] = data
	}
	if err := validateEpicNewLauncherAuthFiles(files); err != nil {
		return fail(err)
	}
	return files, nil
}

func wipeEpicNewLauncherAuthFiles(files map[string][]byte) {
	for _, data := range files {
		clear(data)
	}
}

func validEpicNewLauncherAuthFileName(name string) bool {
	return name != "" &&
		name != "." &&
		name != ".." &&
		filepath.Base(name) == name &&
		!strings.ContainsAny(name, `/\\:`) &&
		!strings.ContainsRune(name, 0)
}

func validateEpicNewLauncherAuthFiles(files map[string][]byte) error {
	if len(files) == 0 || len(files) > epicNewLauncherMaxAuthFiles {
		return errors.New("new Epic launcher auth data is empty or too large")
	}
	total := 0
	for name, data := range files {
		if !validEpicNewLauncherAuthFileName(name) {
			return errors.New("invalid filename in saved Epic launcher auth data")
		}
		if len(data) > epicNewLauncherMaxAuthFileSize {
			return fmt.Errorf("saved Epic launcher auth file %q exceeds the size limit", name)
		}
		total += len(data)
		if total > epicNewLauncherMaxAuthTotalSize {
			return errors.New("saved Epic launcher auth data exceeds the size limit")
		}
	}
	tokenData, ok := files[epicNewLauncherAccessTokenFile]
	if !ok || len(tokenData) == 0 {
		return errors.New("new Epic launcher access-token file is missing")
	}
	authInfo, ok := files[epicNewLauncherAuthInfoFile]
	if !ok || !json.Valid(authInfo) {
		return errors.New("new Epic launcher auth.json is missing or invalid")
	}
	return nil
}

func epicNewLauncherAccountID(files map[string][]byte) (string, error) {
	if err := validateEpicNewLauncherAuthFiles(files); err != nil {
		return "", err
	}
	tokenJSON, err := winutil.DPAPIUnprotect(files[epicNewLauncherAccessTokenFile])
	if err != nil {
		return "", fmt.Errorf("decrypt new Epic launcher access token: %w", err)
	}
	defer clear(tokenJSON)
	return parseEpicNewLauncherAccountID(tokenJSON)
}

func parseEpicNewLauncherAccountID(tokenJSON []byte) (string, error) {
	var token epicNewLauncherToken
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return "", fmt.Errorf("parse new Epic launcher access token: %w", err)
	}
	id := strings.TrimSpace(token.AccountID)
	if id == "" || len(id) > 128 {
		return "", errors.New("new Epic launcher access token does not contain a valid account ID")
	}
	for _, r := range id {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", errors.New("new Epic launcher account ID contains invalid characters")
		}
	}
	return id, nil
}

func readCurrentEpicNewLauncherSession() (*epicNewLauncherSnapshot, error) {
	credential, err := winutil.ReadCredential(epicNewLauncherCredentialTarget)
	if err != nil {
		return nil, fmt.Errorf("read new Epic launcher credential: %w", err)
	}
	defer clear(credential.Blob)
	if !winutil.CredentialTargetMatches(epicNewLauncherCredentialTarget, credential.TargetName) {
		return nil, errors.New("the stored Epic launcher credential has an unexpected target")
	}
	if len(credential.Blob) == 0 {
		return nil, errors.New("new Epic launcher credential is empty; sign in to the launcher first")
	}

	files, err := readEpicNewLauncherAuthFiles()
	if err != nil {
		return nil, err
	}
	id, err := epicNewLauncherAccountID(files)
	if err != nil {
		wipeEpicNewLauncherAuthFiles(files)
		return nil, err
	}

	snapshot := &epicNewLauncherSnapshot{
		Version:  epicNewLauncherSnapshotVersion,
		UniqueID: id,
		Credential: epicNewLauncherCredentialSnapshot{
			TargetName:  credential.TargetName,
			UserName:    credential.UserName,
			TargetAlias: credential.TargetAlias,
			Comment:     credential.Comment,
			Persist:     credential.Persist,
			Blob:        append([]byte(nil), credential.Blob...),
		},
		AuthFiles: files,
	}
	return snapshot, nil
}

func epicNewLauncherCurrentUniqueID() (string, error) {
	snapshot, err := readCurrentEpicNewLauncherSession()
	if err != nil {
		return "", err
	}
	defer wipeEpicNewLauncherSnapshot(snapshot)
	return snapshot.UniqueID, nil
}

func saveEpicNewLauncherSnapshot(destRoot, expectedID string) error {
	snapshot, err := readCurrentEpicNewLauncherSession()
	if err != nil {
		return err
	}
	defer wipeEpicNewLauncherSnapshot(snapshot)
	if !strings.EqualFold(strings.TrimSpace(snapshot.UniqueID), strings.TrimSpace(expectedID)) {
		return errors.New("the new Epic launcher account changed while it was being saved; try again")
	}

	plain, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode new Epic launcher snapshot: %w", err)
	}
	defer clear(plain)
	protected, err := winutil.DPAPIProtect(plain)
	if err != nil {
		return fmt.Errorf("protect new Epic launcher snapshot: %w", err)
	}
	defer clear(protected)

	path := filepath.Join(destRoot, epicNewLauncherSnapshotFile)
	if err := fsutil.WriteFileAtomic(path, protected, 0o600); err != nil {
		return fmt.Errorf("save new Epic launcher snapshot: %w", err)
	}
	return nil
}

func restoreEpicNewLauncherSnapshot(srcRoot, expectedID string) error {
	path := filepath.Join(srcRoot, epicNewLauncherSnapshotFile)
	protected, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("this Epic profile has no new-launcher session yet; sign in to it in Epic Games Launcher, then save the session again")
		}
		return fmt.Errorf("read saved new Epic launcher snapshot: %w", err)
	}
	plain, err := winutil.DPAPIUnprotect(protected)
	clear(protected)
	if err != nil {
		return fmt.Errorf("decrypt saved new Epic launcher snapshot: %w", err)
	}
	defer clear(plain)

	var snapshot epicNewLauncherSnapshot
	if err := json.Unmarshal(plain, &snapshot); err != nil {
		return fmt.Errorf("parse saved new Epic launcher snapshot: %w", err)
	}
	defer wipeEpicNewLauncherSnapshot(&snapshot)

	if snapshot.Version != epicNewLauncherSnapshotVersion {
		return fmt.Errorf("unsupported new Epic launcher snapshot version %d", snapshot.Version)
	}
	if !strings.EqualFold(strings.TrimSpace(snapshot.UniqueID), strings.TrimSpace(expectedID)) {
		return errors.New("saved new Epic launcher profile does not match the selected account")
	}
	if !winutil.CredentialTargetMatches(epicNewLauncherCredentialTarget, snapshot.Credential.TargetName) {
		return errors.New("saved Epic launcher credential target is invalid")
	}
	if len(snapshot.Credential.Blob) == 0 {
		return errors.New("saved Epic launcher credential is empty")
	}
	if snapshot.Credential.Persist < 1 || snapshot.Credential.Persist > 3 {
		return errors.New("saved Epic launcher credential has invalid persistence metadata")
	}
	id, err := epicNewLauncherAccountID(snapshot.AuthFiles)
	if err != nil {
		return err
	}
	if !strings.EqualFold(id, strings.TrimSpace(expectedID)) {
		return errors.New("saved Epic launcher token does not match the selected account")
	}

	credential := winutil.Credential{
		TargetName:  snapshot.Credential.TargetName,
		UserName:    snapshot.Credential.UserName,
		TargetAlias: snapshot.Credential.TargetAlias,
		Comment:     snapshot.Credential.Comment,
		Persist:     snapshot.Credential.Persist,
		Blob:        append([]byte(nil), snapshot.Credential.Blob...),
	}
	defer clear(credential.Blob)
	return installEpicNewLauncherSession(credential, snapshot.AuthFiles)
}

// installEpicNewLauncherSession stages replacement files first, then backs up
// existing regular auth files. Unrecognized directories and non-regular entries
// are left untouched rather than being recursively deleted.
func installEpicNewLauncherSession(credential winutil.Credential, files map[string][]byte) error {
	if !winutil.CredentialTargetMatches(epicNewLauncherCredentialTarget, credential.TargetName) {
		return errors.New("refusing to write an unexpected Epic launcher credential target")
	}
	if len(credential.Blob) == 0 || len(credential.Blob) > 2560 {
		return errors.New("saved Epic launcher credential has an invalid size")
	}
	if credential.Persist < 1 || credential.Persist > 3 {
		return errors.New("saved Epic launcher credential has invalid persistence metadata")
	}
	if err := validateEpicNewLauncherAuthFiles(files); err != nil {
		return err
	}
	authDir, err := epicNewLauncherAuthDir()
	if err != nil {
		return err
	}
	parent := filepath.Dir(authDir)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create new Epic launcher auth parent: %w", err)
	}
	stageDir, err := os.MkdirTemp(parent, ".tcno-epic-stage-")
	if err != nil {
		return fmt.Errorf("stage new Epic launcher auth: %w", err)
	}
	defer os.RemoveAll(stageDir)
	for name, data := range files {
		if !validEpicNewLauncherAuthFileName(name) {
			return errors.New("saved Epic launcher auth data contains an invalid filename")
		}
		if err := os.WriteFile(filepath.Join(stageDir, name), data, 0o600); err != nil {
			return fmt.Errorf("stage new Epic launcher auth file: %w", err)
		}
	}

	info, err := os.Lstat(authDir)
	switch {
	case err == nil:
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("new Epic launcher auth path is not a directory")
		}
	case os.IsNotExist(err):
		if err := os.Mkdir(authDir, 0o700); err != nil {
			return fmt.Errorf("create new Epic launcher auth folder: %w", err)
		}
	default:
		return fmt.Errorf("inspect current Epic launcher auth: %w", err)
	}

	backupDir, err := os.MkdirTemp(parent, ".tcno-epic-backup-")
	if err != nil {
		return fmt.Errorf("prepare auth rollback: %w", err)
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(backupDir)
		}
	}()

	var backedUp, installed []string
	rollback := func(cause error) error {
		var rollbackErrs []string
		for _, name := range installed {
			if err := os.Remove(filepath.Join(authDir, name)); err != nil && !os.IsNotExist(err) {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("remove %s: %v", name, err))
			}
		}
		for i := len(backedUp) - 1; i >= 0; i-- {
			name := backedUp[i]
			if err := os.Rename(filepath.Join(backupDir, name), filepath.Join(authDir, name)); err != nil {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("restore %s: %v", name, err))
			}
		}
		if len(rollbackErrs) > 0 {
			keepBackup = true
			return fmt.Errorf("%w (auth rollback errors: %s; backup kept at %s)", cause, strings.Join(rollbackErrs, "; "), backupDir)
		}
		return cause
	}

	current, err := os.ReadDir(authDir)
	if err != nil {
		return fmt.Errorf("read current Epic launcher auth: %w", err)
	}
	for _, entry := range current {
		fileInfo, err := entry.Info()
		if err != nil {
			return rollback(fmt.Errorf("inspect current Epic auth entry: %w", err))
		}
		if !fileInfo.Mode().IsRegular() {
			continue
		}
		name := entry.Name()
		if !validEpicNewLauncherAuthFileName(name) {
			return rollback(errors.New("unexpected filename in new Epic launcher auth folder"))
		}
		if err := os.Rename(filepath.Join(authDir, name), filepath.Join(backupDir, name)); err != nil {
			return rollback(fmt.Errorf("backup current Epic launcher auth file: %w", err))
		}
		backedUp = append(backedUp, name)
	}
	stageEntries, err := os.ReadDir(stageDir)
	if err != nil {
		return rollback(fmt.Errorf("read staged Epic launcher auth: %w", err))
	}
	for _, entry := range stageEntries {
		name := entry.Name()
		if err := os.Rename(filepath.Join(stageDir, name), filepath.Join(authDir, name)); err != nil {
			return rollback(fmt.Errorf("install new Epic launcher auth file: %w", err))
		}
		installed = append(installed, name)
	}

	if err := winutil.WriteCredential(credential); err != nil {
		return rollback(fmt.Errorf("write new Epic launcher credential: %w", err))
	}
	if err := os.RemoveAll(backupDir); err != nil {
		keepBackup = true
		return fmt.Errorf("Epic launcher session restored, but old auth files could not be removed from %s: %w", backupDir, err)
	}
	return nil
}

func clearEpicNewLauncherSession() error {
	authDir, err := epicNewLauncherAuthDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(authDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read new Epic launcher auth folder: %w", err)
	}

	parent := filepath.Dir(authDir)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create Epic launcher auth parent: %w", err)
	}
	backupDir, err := os.MkdirTemp(parent, ".tcno-epic-clear-")
	if err != nil {
		return fmt.Errorf("prepare Epic launcher auth backup: %w", err)
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(backupDir)
		}
	}()

	var moved []string
	rollback := func(cause error) error {
		var rollbackErrs []string
		for i := len(moved) - 1; i >= 0; i-- {
			name := moved[i]
			if err := os.Rename(filepath.Join(backupDir, name), filepath.Join(authDir, name)); err != nil {
				rollbackErrs = append(rollbackErrs, fmt.Sprintf("restore %s: %v", name, err))
			}
		}
		if len(rollbackErrs) > 0 {
			keepBackup = true
			return fmt.Errorf("%w (auth rollback errors: %s; backup kept at %s)", cause, strings.Join(rollbackErrs, "; "), backupDir)
		}
		return cause
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return rollback(fmt.Errorf("inspect new Epic launcher auth entry: %w", err))
		}
		if !info.Mode().IsRegular() {
			continue
		}
		name := entry.Name()
		if !validEpicNewLauncherAuthFileName(name) {
			return rollback(errors.New("unexpected filename in new Epic launcher auth folder"))
		}
		if err := os.Rename(filepath.Join(authDir, name), filepath.Join(backupDir, name)); err != nil {
			return rollback(fmt.Errorf("move current Epic launcher auth file aside: %w", err))
		}
		moved = append(moved, name)
	}

	if err := winutil.DeleteCredential(epicNewLauncherCredentialTarget); err != nil && !errors.Is(err, winutil.ErrCredentialNotFound) {
		return rollback(fmt.Errorf("remove new Epic launcher credential: %w", err))
	}
	if err := os.RemoveAll(backupDir); err != nil {
		keepBackup = true
		return fmt.Errorf("Epic launcher session was cleared, but old auth files could not be removed from %s: %w", backupDir, err)
	}
	return nil
}

func wipeEpicNewLauncherSnapshot(snapshot *epicNewLauncherSnapshot) {
	if snapshot == nil {
		return
	}
	clear(snapshot.Credential.Blob)
	for _, data := range snapshot.AuthFiles {
		clear(data)
	}
}

