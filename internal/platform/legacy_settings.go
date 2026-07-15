package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/settingsfile"
)

const legacyWindowSettingsFileName = "WindowSettings.json"

type legacyWindowSettings struct {
	DisabledPlatforms     []string `json:"DisabledPlatforms"`
	EnabledBasicPlatforms []string `json:"EnabledBasicPlatforms"`
	CollectStats          *bool    `json:"CollectStats"`
	ShareAnonymousStats   *bool    `json:"ShareAnonymousStats"`
}

func migrateLegacyWindowSettings(exeDir string) (AppSettings, string, bool, error) {
	for _, dir := range settingsfile.DefaultSearchDirs(exeDir) {
		legacyPath := filepath.Join(dir, legacyWindowSettingsFileName)
		data, err := os.ReadFile(legacyPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return AppSettings{}, "", false, err
		}

		var legacy legacyWindowSettings
		if err := json.Unmarshal(data, &legacy); err != nil {
			return AppSettings{}, "", false, err
		}

		settings := defaultSettings()
		if legacy.CollectStats != nil {
			settings.StatsEnabled = *legacy.CollectStats
		}
		if legacy.ShareAnonymousStats != nil {
			settings.StatsShare = *legacy.ShareAnonymousStats
		}

		catalog, err := legacyMigrationCatalog(dir)
		if err != nil {
			return AppSettings{}, "", false, err
		}
		settings.DisabledPlatforms, err = legacyDisabledPlatformNames(catalog, legacy)
		if err != nil {
			return AppSettings{}, "", false, err
		}

		path, err := resolveSettingsSavePath(exeDir, settings)
		if err != nil {
			return AppSettings{}, "", false, err
		}
		encoded, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return AppSettings{}, "", false, err
		}
		if err := atomicWriteBytes(path, encoded, 0o644); err != nil {
			return AppSettings{}, "", false, err
		}
		return settings, path, true, nil
	}
	return AppSettings{}, "", false, nil
}

func legacyMigrationCatalog(userDataDir string) ([]byte, error) {
	path := filepath.Join(userDataDir, "Platforms.json")
	base, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if len(embeddedPlatformsJSON) == 0 {
			return []byte(`{"Platforms":{"Steam":{"Identifiers":["s","steam","valve"]}}}`), nil
		}
		return append([]byte(nil), embeddedPlatformsJSON...), nil
	}

	merged, changed, err := addEmbeddedSteamPlatform(base, embeddedPlatformsJSON)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := atomicWriteBytes(path, merged, 0o644); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

func addEmbeddedSteamPlatform(base, embedded []byte) ([]byte, bool, error) {
	var current platformsFile
	if err := json.Unmarshal(base, &current); err != nil {
		return nil, false, err
	}
	if current.Platforms == nil {
		return nil, false, errors.New("legacy Platforms.json missing Platforms")
	}
	if _, ok := current.Platforms["Steam"]; ok {
		return base, false, nil
	}

	var defaults platformsFile
	if len(embedded) > 0 {
		if err := json.Unmarshal(embedded, &defaults); err != nil {
			return nil, false, err
		}
	}
	steam, ok := defaults.Platforms["Steam"]
	if !ok {
		steam = json.RawMessage(`{"Identifiers":["s","steam","valve"]}`)
	}
	current.Platforms["Steam"] = steam
	merged, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return merged, true, nil
}

func legacyDisabledPlatformNames(catalog []byte, legacy legacyWindowSettings) ([]string, error) {
	var file platformsFile
	if err := json.Unmarshal(catalog, &file); err != nil {
		return nil, err
	}
	if file.Platforms == nil {
		return nil, errors.New("legacy Platforms.json missing Platforms")
	}

	enabledBasic := normalizedStringSet(legacy.EnabledBasicPlatforms)
	legacyDisabled := normalizedStringSet(legacy.DisabledPlatforms)
	disabled := make([]string, 0, len(file.Platforms))
	for name, raw := range file.Platforms {
		var descriptor Descriptor
		if err := json.Unmarshal(raw, &descriptor); err != nil {
			return nil, err
		}
		aliases := append([]string{name}, descriptor.Identifiers...)
		enabled := false
		if strings.EqualFold(name, "Steam") {
			enabled = !containsNormalizedAlias(legacyDisabled, aliases)
		} else {
			enabled = containsNormalizedAlias(enabledBasic, aliases)
			if containsNormalizedAlias(legacyDisabled, aliases) {
				enabled = false
			}
		}
		if !enabled {
			disabled = append(disabled, name)
		}
	}
	sort.Slice(disabled, func(i, j int) bool {
		return strings.ToLower(disabled[i]) < strings.ToLower(disabled[j])
	})
	return disabled, nil
}

func normalizedStringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func containsNormalizedAlias(values map[string]struct{}, aliases []string) bool {
	for _, alias := range aliases {
		if _, ok := values[strings.ToLower(strings.TrimSpace(alias))]; ok {
			return true
		}
	}
	return false
}

func applyStatsInstallerPreference(settings *AppSettings) (bool, error) {
	dir, err := DefaultUserDataDir()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(filepath.Join(dir, "SendAnonymousStats.no")); err == nil {
		settings.StatsEnabled = false
		settings.StatsShare = false
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if _, err := os.Stat(filepath.Join(dir, "SendAnonymousStats.yes")); err == nil {
		settings.StatsEnabled = true
		settings.StatsShare = true
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return false, nil
}

func clearStatsInstallerPreference() {
	dir, err := DefaultUserDataDir()
	if err != nil {
		return
	}
	_ = os.Remove(filepath.Join(dir, "SendAnonymousStats.no"))
	_ = os.Remove(filepath.Join(dir, "SendAnonymousStats.yes"))
}
