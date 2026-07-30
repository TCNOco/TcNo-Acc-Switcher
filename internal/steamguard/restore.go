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
)

var (
	ErrRestoreRequiresEmptyVault = errors.New("Steam Guard restore requires an empty vault")
	ErrRestoreInProgress         = errors.New("a Steam Guard restore is in progress")
)

// RestoreSourceInfo describes a chosen backup folder before any password is
// collected, so the caller only asks for the passwords the folder actually needs.
type RestoreSourceInfo struct {
	HasOuterLayer bool `json:"hasOuterLayer"`
}

// InspectRestoreBackup reports what the folder needs to be restored. It only
// reads: a folder the user picked is theirs, and must not be modified by being
// looked at.
func (s *Service) InspectRestoreBackup(source string) (RestoreSourceInfo, error) {
	canonicalSource, err := canonicalRestoreSource(source)
	if err != nil {
		return RestoreSourceInfo{}, err
	}
	info, err := vault.Inspect(canonicalSource)
	if err != nil {
		return RestoreSourceInfo{}, err
	}
	return RestoreSourceInfo{HasOuterLayer: info.HasRecoveryWrapper}, nil
}

// RestoreVerifiedBackup restores a self-contained encrypted Steam Guard folder
// from source. A backup app password is required only when the copied vault has
// an outer layer; the current app password is required only when this
// installation has app protection enabled.
func (s *Service) RestoreVerifiedBackup(source, steamGuardPassword, backupAppPassword, currentAppPassword string) (string, error) {
	return s.RestoreVerifiedBackupWithFactors(source, steamGuardPassword, backupAppPassword, currentAppPassword, "", "")
}

// RestoreVerifiedBackupWithFactors restores a backup whose slots need more than
// a password. The factors are the ones enrolled in the backup, which are also
// the ones the restored vault ends up with.
func (s *Service) RestoreVerifiedBackupWithFactors(source, steamGuardPassword, backupAppPassword, currentAppPassword, keyfilePath, backupKey string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", ErrInvalidBackupDestination
	}
	creds, err := buildVaultCredentials(steamGuardPassword, keyfilePath, backupKey)
	if err != nil {
		return "", err
	}
	defer wipe(creds.Keyfile)
	defer wipe(creds.RecoveryCode)
	return s.restoreVerifiedBackupAtWith(source, creds, backupAppPassword, currentAppPassword)
}

func canonicalRestoreSource(source string) (string, error) {
	source = filepath.Clean(strings.TrimSpace(source))
	if source == "." || !filepath.IsAbs(source) {
		return "", ErrInvalidBackupDestination
	}
	canonical, err := canonicalSafeDirectory(source)
	if err != nil {
		return "", errors.Join(ErrInvalidBackupDestination, err)
	}
	return canonical, nil
}

func (s *Service) restoreVerifiedBackupAt(source, steamGuardPassword, backupAppPassword, currentAppPassword string) (string, error) {
	return s.restoreVerifiedBackupAtWith(source, vault.PasswordOnly(steamGuardPassword), backupAppPassword, currentAppPassword)
}

func (s *Service) restoreVerifiedBackupAtWith(source string, creds vault.Credentials, backupAppPassword, currentAppPassword string) (string, error) {
	canonicalSource, err := canonicalRestoreSource(source)
	if err != nil {
		return "", err
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
	// From here the live vault path exists but is not yet a vault. The lock is
	// released further down to ask a security key enrolled in the backup, and
	// anything that opened this folder in that window would hold a vault the
	// failure path below deletes out from under it.
	s.restoreInProgress = true
	committed := false
	defer func() {
		s.restoreInProgress = false
		if !committed {
			// Nothing may keep a handle on a folder that is about to go.
			s.vault = nil
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
	// Asked once the backup's own header is readable, so a key enrolled in the
	// backup can open it. Checked here rather than up front because whether
	// anything was supplied is not knowable until the copy says which factors it
	// wants: on a key-only vault the user has nothing to type.
	s.fillSecurityKeyLocked(restored, &creds)
	defer wipe(creds.SecurityKey)
	if !credentialsSupplied(creds) {
		return "", vault.ErrInvalidPassword
	}
	hadOuter := restored.HasRecoveryWrapper()
	if err := verifyCopiedVault(destination, creds, backupAppPassword, hadOuter, s.vaultOptions); err != nil {
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
		err = restored.UnlockWithFactorsAndOuter(creds, currentOuterKey, vault.FixedLease)
	} else {
		err = restored.UnlockWith(creds, vault.FixedLease)
	}
	if err != nil {
		return "", err
	}
	// The copy carries the backup's deliberately expensive parameters. It is
	// now the live vault and will be unlocked routinely, so bring the cost back
	// down; leaving it would make every future unlock pay the backup rate. The
	// lease taken above supplies the outer key, and this is another journalled
	// generation, so an interruption is recovered on the next Open.
	if err := restored.RekeyWith(creds, s.liveKDFParams()); err != nil {
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
