package steamguard

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/securefile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	backupNamePrefix = "TcNo-Acc-Switcher-SteamGuard-Backup-"
	backupMaxFiles   = 4096
	backupMaxFile    = int64(64 << 20)
	backupMaxTotal   = int64(1 << 30)
)

var (
	ErrInvalidBackupDestination = errors.New("invalid Steam Guard backup destination")
	ErrBackupSourceChanged      = errors.New("Steam Guard backup source changed during copy")
	ErrBackupLimit              = errors.New("Steam Guard backup exceeds safety limits")
)

// CreateVerifiedBackup copies the encrypted vault into a new protected folder,
// then independently opens and decrypts every copied record before reporting success.
func (s *Service) CreateVerifiedBackup(steamGuardPassword, appPassword string) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("application not initialised")
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Choose a folder for the encrypted Steam Guard backup").
		CanChooseDirectories(true).
		CanChooseFiles(false)
	if owner := dialogOwnerWindow(); owner != nil {
		dialog = dialog.AttachToWindow(owner)
	}
	parent, err := dialog.PromptForSingleSelection()
	logDialogOutcome("steam-guard-backup-folder", strings.TrimSpace(parent) != "", err)
	if err != nil {
		if dialogCancelled(err) {
			// Cancel is a clean outcome: an empty path with no error.
			return "", nil
		}
		return "", err
	}
	if strings.TrimSpace(parent) == "" {
		return "", nil
	}
	return s.createVerifiedBackupAt(parent, steamGuardPassword, appPassword, time.Now().UTC())
}

func (s *Service) createVerifiedBackupAt(parent, steamGuardPassword, appPassword string, now time.Time) (string, error) {
	if steamGuardPassword == "" {
		return "", vault.ErrInvalidPassword
	}
	parent = filepath.Clean(strings.TrimSpace(parent))
	if !filepath.IsAbs(parent) {
		return "", ErrInvalidBackupDestination
	}
	canonicalParent, err := canonicalSafeDirectory(parent)
	if err != nil {
		return "", errors.Join(ErrInvalidBackupDestination, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.requireVaultLocked(); err != nil {
		return "", err
	}
	source, err := VaultFolderPath()
	if err != nil {
		return "", err
	}
	canonicalSource, err := canonicalSafeDirectory(source)
	if err != nil {
		return "", err
	}
	if withinPath(canonicalSource, canonicalParent) {
		return "", ErrInvalidBackupDestination
	}

	securityStatus, err := securityStatusForBackup()
	if err != nil {
		return "", err
	}
	if securityStatus && appPassword == "" {
		return "", ErrAppPassword
	}

	destination, err := createBackupRoot(canonicalParent, now)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			removeCreatedBackup(canonicalParent, destination)
		}
	}()

	if err := copyBackupTree(canonicalSource, destination); err != nil {
		return "", err
	}
	if err := verifyCopiedVault(destination, steamGuardPassword, appPassword, securityStatus, s.vaultOptions); err != nil {
		return "", err
	}
	settings, err := LoadSettings()
	if err != nil {
		return "", err
	}
	settings.LastVerifiedBackup = now.UTC().Format(time.RFC3339)
	settings.LastVerifiedBackupPath = destination
	if err := SaveSettings(settings); err != nil {
		return "", err
	}
	committed = true
	return destination, nil
}

func securityStatusForBackup() (bool, error) {
	status, err := security.GetStatus()
	if err != nil {
		return false, err
	}
	return status.SavedAccountDataEncrypted, nil
}

func canonicalSafeDirectory(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return "", ErrInvalidBackupDestination
	}
	reparse, err := securefile.IsReparsePoint(path)
	if err != nil || reparse {
		return "", ErrInvalidBackupDestination
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", err
	}
	if !samePath(filepath.Clean(path), filepath.Clean(canonical)) {
		return "", ErrInvalidBackupDestination
	}
	return filepath.Clean(canonical), nil
}

func createBackupRoot(parent string, now time.Time) (string, error) {
	base := backupNamePrefix + now.UTC().Format("20060102-150405")
	for attempt := 0; attempt < 100; attempt++ {
		name := base
		if attempt != 0 {
			name = fmt.Sprintf("%s-%02d", base, attempt)
		}
		destination := filepath.Join(parent, name)
		if err := securefile.CreateDirectoryNew(destination); err == nil {
			return destination, nil
		} else if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fs.ErrExist
}

func copyBackupTree(source, destination string) error {
	var files int
	var total int64
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || !safeRelativePath(rel) {
			return ErrInvalidBackupDestination
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		reparse, err := securefile.IsReparsePoint(path)
		if err != nil || reparse || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidBackupDestination
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destination, rel)
		if info.IsDir() {
			return securefile.CreateDirectoryNew(target)
		}
		if !info.Mode().IsRegular() {
			return ErrInvalidBackupDestination
		}
		files++
		if files > backupMaxFiles || info.Size() < 0 || info.Size() > backupMaxFile || total > backupMaxTotal-info.Size() {
			return ErrBackupLimit
		}
		total += info.Size()
		return copyBackupFile(path, target, info)
	})
}

func copyBackupFile(source, destination string, expected os.FileInfo) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(expected, opened) || opened.Size() != expected.Size() {
		return ErrBackupSourceChanged
	}
	output, err := securefile.CreateNew(destination)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = output.Close()
			_ = os.Remove(destination)
		}
	}()
	written, err := io.CopyN(output, input, expected.Size())
	if err != nil || written != expected.Size() {
		return errors.Join(ErrBackupSourceChanged, err)
	}
	var extra [1]byte
	if count, readErr := input.Read(extra[:]); count != 0 || !errors.Is(readErr, io.EOF) {
		return ErrBackupSourceChanged
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}

func verifyCopiedVault(path, steamGuardPassword, appPassword string, appPasswordSet bool, options []vault.Option) error {
	copyVault, err := vault.Open(path, options...)
	if err != nil {
		return err
	}
	if appPasswordSet {
		if !copyVault.HasRecoveryWrapper() {
			return vault.ErrInvalidFormat
		}
		return copyVault.VerifyRecovery(steamGuardPassword, appPassword)
	}
	if copyVault.HasRecoveryWrapper() {
		return vault.ErrOuterKeyRequired
	}
	if err := copyVault.Unlock(steamGuardPassword, vault.FixedLease); err != nil {
		return err
	}
	defer copyVault.Lock()
	records, err := copyVault.List()
	if err != nil {
		return err
	}
	for _, record := range records {
		plaintext, err := copyVault.Get(record.ID)
		wipe(plaintext)
		if err != nil {
			return err
		}
	}
	return nil
}

func safeRelativePath(path string) bool {
	if path == "." {
		return true
	}
	return path != "" && path != ".." && !filepath.IsAbs(path) && !strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func withinPath(parent, candidate string) bool {
	rel, err := filepath.Rel(parent, candidate)
	return err == nil && (rel == "." || safeRelativePath(rel))
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func removeCreatedBackup(parent, destination string) {
	rel, err := filepath.Rel(parent, destination)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.ContainsRune(rel, filepath.Separator) {
		return
	}
	_ = os.RemoveAll(destination)
}
