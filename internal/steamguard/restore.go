package steamguard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
	"TcNo-Acc-Switcher/internal/steamguard/securefile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var ErrRestoreRequiresEmptyVault = errors.New("Steam Guard restore requires an empty vault")

// RestoreVerifiedBackup restores a self-contained encrypted Steam Guard folder.
// A backup app password is required only when the copied vault has an outer
// layer; the current app password is required only when this installation has
// app protection enabled.
func (s *Service) RestoreVerifiedBackup(steamGuardPassword, backupAppPassword, currentAppPassword string) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("application not initialised")
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Choose an encrypted Steam Guard backup folder").
		CanChooseDirectories(true).
		CanChooseFiles(false)
	if owner := dialogOwnerWindow(); owner != nil {
		dialog = dialog.AttachToWindow(owner)
	}
	source, err := dialog.PromptForSingleSelection()
	logDialogOutcome("steam-guard-restore-folder", strings.TrimSpace(source) != "", err)
	if err != nil {
		if dialogCancelled(err) {
			// Cancel is a clean outcome: an empty path with no error.
			return "", nil
		}
		return "", err
	}
	if strings.TrimSpace(source) == "" {
		return "", nil
	}
	return s.restoreVerifiedBackupAt(source, steamGuardPassword, backupAppPassword, currentAppPassword)
}

func (s *Service) restoreVerifiedBackupAt(source, steamGuardPassword, backupAppPassword, currentAppPassword string) (string, error) {
	if strings.TrimSpace(steamGuardPassword) == "" {
		return "", vault.ErrInvalidPassword
	}
	source = filepath.Clean(strings.TrimSpace(source))
	if !filepath.IsAbs(source) {
		return "", ErrInvalidBackupDestination
	}
	canonicalSource, err := canonicalSafeDirectory(source)
	if err != nil {
		return "", errors.Join(ErrInvalidBackupDestination, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists, openErr := s.openVaultLocked(); openErr != nil {
		return "", openErr
	} else if exists {
		return "", ErrRestoreRequiresEmptyVault
	}
	destination, err := VaultFolderPath()
	if err != nil {
		return "", err
	}
	if _, statErr := os.Lstat(destination); statErr == nil {
		return "", ErrRestoreRequiresEmptyVault
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	parent, err := canonicalSafeDirectory(filepath.Dir(destination))
	if err != nil {
		return "", err
	}
	if withinPath(canonicalSource, parent) || withinPath(parent, canonicalSource) {
		return "", ErrInvalidBackupDestination
	}
	if err := securefile.CreateDirectoryNew(destination); err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			removeCreatedBackup(parent, destination)
		}
	}()
	if err := copyBackupTree(canonicalSource, destination); err != nil {
		return "", err
	}

	restored, err := vault.Open(destination, s.vaultOptions...)
	if err != nil {
		return "", err
	}
	defer func() {
		if !committed {
			_ = restored.Lock()
		}
	}()
	hadOuter := restored.HasRecoveryWrapper()
	if err := verifyCopiedVault(destination, steamGuardPassword, backupAppPassword, hadOuter, s.vaultOptions); err != nil {
		return "", err
	}
	currentOuterKey, err := appOuterKeyForRecovery(currentAppPassword)
	if err != nil {
		return "", err
	}
	defer security.WipeSecret(currentOuterKey)
	switch {
	case hadOuter && len(currentOuterKey) != 0:
		if err := restored.RestoreOuterFromRecovery(backupAppPassword, currentOuterKey, currentAppPassword); err != nil {
			return "", err
		}
	case hadOuter:
		if err := restored.DisableOuterWithRecovery(backupAppPassword); err != nil {
			return "", err
		}
	case len(currentOuterKey) != 0:
		if err := restored.EnableOuterWithRecovery(currentOuterKey, currentAppPassword); err != nil {
			return "", err
		}
	}
	if len(currentOuterKey) != 0 {
		err = restored.UnlockWithOuter(steamGuardPassword, currentOuterKey, vault.FixedLease)
	} else {
		err = restored.Unlock(steamGuardPassword, vault.FixedLease)
	}
	if err != nil {
		return "", err
	}
	records, err := restored.List()
	if err != nil {
		return "", err
	}
	for _, record := range records {
		plaintext, getErr := restored.Get(record.ID)
		wipe(plaintext)
		if getErr != nil {
			return "", getErr
		}
		if err := registry.Upsert(record.SteamID64, registry.StateActive); err != nil {
			return "", err
		}
	}
	if err := restored.Lock(); err != nil {
		return "", err
	}
	settings, err := LoadSettings()
	if err != nil {
		return "", err
	}
	settings.FeatureEnabled = true
	settings.LastVerifiedBackup = ""
	settings.LastVerifiedBackupPath = ""
	if err := SaveSettings(settings); err != nil {
		return "", err
	}
	s.vault = restored
	committed = true
	return destination, nil
}
