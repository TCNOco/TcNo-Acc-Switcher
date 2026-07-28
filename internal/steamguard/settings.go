package steamguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
)

const (
	settingsVersion = 1
	settingsName    = "SteamGuard.json"
	maxSettingsSize = 64 << 10
)

var ErrInvalidSettings = errors.New("invalid Steam Guard settings")

type Settings struct {
	Version                    int    `json:"version"`
	FeatureEnabled             bool   `json:"featureEnabled"`
	RememberPasswordForSession bool   `json:"rememberPasswordForSession"`
	LastVerifiedBackup         string `json:"lastVerifiedBackup,omitempty"`
	LastVerifiedBackupPath     string `json:"lastVerifiedBackupPath,omitempty"`
}

func defaultSettings() Settings { return Settings{Version: settingsVersion} }

func settingsPath() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "Settings", settingsName), nil
}

func LoadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return Settings{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return Settings{}, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxSettingsSize+1))
	if err != nil {
		return Settings{}, err
	}
	if len(raw) > maxSettingsSize {
		return Settings{}, ErrInvalidSettings
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var settings Settings
	if err := dec.Decode(&settings); err != nil {
		return Settings{}, errors.Join(ErrInvalidSettings, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF || settings.Version != settingsVersion {
		return Settings{}, ErrInvalidSettings
	}
	if settings.LastVerifiedBackup != "" {
		if _, err := time.Parse(time.RFC3339, settings.LastVerifiedBackup); err != nil {
			return Settings{}, ErrInvalidSettings
		}
	}
	return settings, nil
}

func SaveSettings(settings Settings) error {
	settings.Version = settingsVersion
	settings.LastVerifiedBackup = strings.TrimSpace(settings.LastVerifiedBackup)
	settings.LastVerifiedBackupPath = strings.TrimSpace(settings.LastVerifiedBackupPath)
	if settings.LastVerifiedBackup == "" {
		settings.LastVerifiedBackupPath = ""
	}
	if settings.LastVerifiedBackup != "" {
		parsed, err := time.Parse(time.RFC3339, settings.LastVerifiedBackup)
		if err != nil {
			return ErrInvalidSettings
		}
		settings.LastVerifiedBackup = parsed.UTC().Format(time.RFC3339)
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	path, err := settingsPath()
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(raw, '\n'), 0o600)
}

func VaultFolderPath() (string, error) { return registry.RootPath() }
