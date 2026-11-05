package basic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/winutil"
)

func RegisterCredentialSessionBackups(service *BasicService) {
	if service == nil {
		platform.SetCredentialSessionBackupHandlers(nil, nil)
		return
	}
	platform.SetCredentialSessionBackupHandlers(
		func(platformKey string) (platform.BackupResult, error) {
			return backupCredentialSession(service, platformKey)
		},
		func(platformKey string) (platform.BackupResult, error) {
			return restoreCredentialSessionBackup(service, platformKey)
		},
	)
}

func backupCredentialSession(service *BasicService, platformKey string) (platform.BackupResult, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return platform.BackupResult{}, err
	}
	fc, err := PrepareFlow(service.deps(), platformKey)
	if err != nil {
		return platform.BackupResult{}, err
	}
	cfg, err := credentialSessionConfiguration(fc)
	if err != nil {
		return platform.BackupResult{}, err
	}
	if err := killPlatformExes(service.deps(), fc); err != nil {
		return platform.BackupResult{}, err
	}
	recovery := &credentialSessionRecovery{Version: credentialSessionVersion}
	defer wipeCredentialSessionRecovery(recovery)
	credentials, err := winutil.ReadCredentials(cfg.target)
	defer func() {
		for i := range credentials {
			clear(credentials[i].Blob)
		}
	}()
	if err == nil {
		recovery.Credentials = credentials
		for i := range recovery.Credentials {
			if err := validateSessionCredential(cfg.target, &recovery.Credentials[i]); err != nil {
				return platform.BackupResult{}, err
			}
		}
		if len(recovery.Credentials) > 0 {
			recovery.Credential = &recovery.Credentials[0]
		}
	} else if !errors.Is(err, winutil.ErrCredentialNotFound) {
		return platform.BackupResult{}, err
	}
	recovery.AuthFiles, err = readCredentialSessionFiles(cfg, true)
	if err != nil {
		return platform.BackupResult{}, err
	}
	path, err := saveCredentialSessionRecovery(cfg, recovery)
	if err != nil {
		return platform.BackupResult{}, err
	}
	if err := pruneCredentialSessionRecovery(cfg); err != nil {
		logFlow().Warn("session backup retention failed", "platform", platformKey, "err", err)
	}
	return credentialSessionBackupResult(path, recovery), nil
}

func restoreCredentialSessionBackup(service *BasicService, platformKey string) (platform.BackupResult, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return platform.BackupResult{}, err
	}
	fc, err := PrepareFlow(service.deps(), platformKey)
	if err != nil {
		return platform.BackupResult{}, err
	}
	cfg, err := credentialSessionConfiguration(fc)
	if err != nil {
		return platform.BackupResult{}, err
	}
	path, recovery, err := latestCredentialSessionRecovery(cfg)
	if err != nil {
		return platform.BackupResult{}, err
	}
	defer wipeCredentialSessionRecovery(recovery)
	if err := killPlatformExes(service.deps(), fc); err != nil {
		return platform.BackupResult{}, err
	}
	if err := applyCredentialSessionRecovery(cfg, recovery); err != nil {
		return platform.BackupResult{}, err
	}
	recovery.Incomplete = false
	if err := writeProtectedSession(path, recovery); err != nil {
		logFlow().Warn("session restored but recovery record could not be finalized", "platform", platformKey, "err", err)
	}
	if err := pruneCredentialSessionRecovery(cfg); err != nil {
		logFlow().Warn("session recovery retention failed", "platform", platformKey, "err", err)
	}
	result := credentialSessionBackupResult(path, recovery)
	result.RestoredFrom = path
	return result, nil
}

func latestCredentialSessionRecovery(cfg credentialSessionConfig) (string, *credentialSessionRecovery, error) {
	directory, err := platform.CredentialSessionBackupDirectory(cfg.platformKey)
	if err != nil {
		return "", nil, err
	}
	if err := validateCredentialSessionPath(directory); err != nil {
		return "", nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", nil, fmt.Errorf("read session backups: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(strings.ToLower(entry.Name()), ".dpapi") {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	var newestError error
	for _, name := range names {
		path := filepath.Join(directory, name)
		recovery, err := readCredentialSessionRecovery(path, cfg)
		if err == nil {
			return path, recovery, nil
		}
		if newestError == nil {
			newestError = err
		}
	}
	if newestError != nil {
		return "", nil, fmt.Errorf("no valid session backup found: %w", newestError)
	}
	return "", nil, errors.New("no session backup found")
}

func credentialSessionBackupResult(path string, recovery *credentialSessionRecovery) platform.BackupResult {
	result := platform.BackupResult{ArchivePath: path, Files: len(recovery.AuthFiles)}
	for _, data := range recovery.AuthFiles {
		result.Bytes += int64(len(data))
	}
	if len(recovery.Credentials) > 0 {
		for _, credential := range recovery.Credentials {
			result.Files++
			result.Bytes += int64(len(credential.Blob))
		}
	} else if recovery.Credential != nil {
		result.Files++
		result.Bytes += int64(len(recovery.Credential.Blob))
	}
	return result
}
