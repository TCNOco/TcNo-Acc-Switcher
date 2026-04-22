package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"TcNo-Acc-Switcher/internal/fsutil"

	"github.com/tidwall/gjson"
)

// GameShortcutEntry is one cached game shortcut (.lnk / .url) in the footer bar or dropdown.
type GameShortcutEntry struct {
	FileName string `json:"fileName"`
	Pinned   bool   `json:"pinned"`
}

// UnmarshalJSON accepts legacy / interop casing (e.g. C# FileName, Pinned) so shortcuts are not dropped.
func (e *GameShortcutEntry) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*e = GameShortcutEntry{}
	for _, k := range []string{"fileName", "FileName", "file_name"} {
		if v, ok := m[k].(string); ok {
			e.FileName = strings.TrimSpace(v)
			if e.FileName != "" {
				break
			}
		}
	}
	for _, k := range []string{"pinned", "Pinned"} {
		switch v := m[k].(type) {
		case bool:
			e.Pinned = v
			return nil
		case float64:
			e.Pinned = v != 0
			return nil
		case json.Number:
			if i, err := v.Int64(); err == nil {
				e.Pinned = i != 0
				return nil
			}
		case string:
			if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
				e.Pinned = b
				return nil
			}
		}
	}
	return nil
}

// dataDirName matches [paths.DataDirName] (cannot import paths: it imports platform).
const dataDirName = "TcNo Account Switcher"

func settingsDirUnderExe() (string, error) {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(exeDir, dataDirName, "Settings"), nil
}

// PlatformSettings are shared per-platform fields stored in Settings/<Name>Settings.json
type PlatformSettings struct {
	RunAsAdmin           bool                `json:"RunAsAdmin"`
	TrayAccNumber        int                 `json:"TrayAccNumber"`
	ForgetAccountEnabled bool                `json:"ForgetAccountEnabled"`
	ClosingMethod        string              `json:"ClosingMethod"`
	StartingMethod       string              `json:"StartingMethod"`
	AutoStart            bool                `json:"AutoStart"`
	ShowShortNotes       bool                `json:"ShowShortNotes"`
	AccountNotes         map[string]string   `json:"AccountNotes"`
	Shortcuts            []GameShortcutEntry `json:"Shortcuts,omitempty"`
	AlwaysSwapOnShortcut bool                `json:"AlwaysSwapOnShortcut,omitempty"`
}

// DefaultPlatformSettings returns defaults for a new platform settings file.
func DefaultPlatformSettings() PlatformSettings {
	return PlatformSettings{
		TrayAccNumber:        3,
		ClosingMethod:        "Combined",
		StartingMethod:       "Default",
		AutoStart:            true,
		ShowShortNotes:       true,
		AccountNotes:         map[string]string{},
		Shortcuts:            []GameShortcutEntry{},
		ForgetAccountEnabled: false,
		RunAsAdmin:           false,
		AlwaysSwapOnShortcut: true,
	}
}

func sanitizePlatformSettingsFilePrefix(platformKey string) string {
	s := strings.TrimSpace(platformKey)
	s = strings.ReplaceAll(s, " ", "")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 32 || strings.ContainsRune(`<>:"/\|?*`, r):
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "Platform"
	}
	return out
}

func platformSettingsJSONPath(platformKey string) (string, error) {
	dir, err := settingsDirUnderExe()
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(platformKey)
	if strings.EqualFold(key, "Steam") {
		return filepath.Join(dir, "SteamSettings.json"), nil
	}
	return filepath.Join(dir, sanitizePlatformSettingsFilePrefix(key)+"Settings.json"), nil
}

// LoadPlatformSettings reads common platform settings from the per-platform JSON file.
// For Steam, reads SteamSettings.json and unmarshals only matching keys.
func LoadPlatformSettings(platformKey string) (PlatformSettings, error) {
	path, err := platformSettingsJSONPath(platformKey)
	if err != nil {
		return PlatformSettings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPlatformSettings(), nil
		}
		return PlatformSettings{}, err
	}
	var s PlatformSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultPlatformSettings(), err
	}
	if gjson.GetBytes(data, "AlwaysSwapOnShortcut").Exists() {
		s.AlwaysSwapOnShortcut = gjson.GetBytes(data, "AlwaysSwapOnShortcut").Bool()
	} else {
		s.AlwaysSwapOnShortcut = true
	}
	if s.AccountNotes == nil {
		s.AccountNotes = map[string]string{}
	}
	if s.TrayAccNumber <= 0 {
		s.TrayAccNumber = 3
	}
	if strings.TrimSpace(s.ClosingMethod) == "" {
		s.ClosingMethod = "Combined"
	}
	if strings.TrimSpace(s.StartingMethod) == "" {
		s.StartingMethod = "Default"
	}
	if s.Shortcuts == nil {
		s.Shortcuts = []GameShortcutEntry{}
	}
	return s, nil
}

// SavePlatformSettings merges common fields into the per-platform JSON file without
// removing other keys (e.g. Steam-specific fields in SteamSettings.json).
func SavePlatformSettings(platformKey string, s PlatformSettings) error {
	path, err := platformSettingsJSONPath(platformKey)
	if err != nil {
		return err
	}
	if s.AccountNotes == nil {
		s.AccountNotes = map[string]string{}
	}
	existing := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &existing)
	}
	patch, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var patchMap map[string]any
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return err
	}
	for k, v := range patchMap {
		existing[k] = v
	}
	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, out, 0o644)
}

// resetPlatformJSONToDefaults overwrites the per-platform JSON with defaults (common fields only).
// For Steam, use the hook registered via SetSteamReset (full Steam defaults).
func resetPlatformJSONToDefaults(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	if strings.EqualFold(platformKey, "Steam") {
		if resetSteamSettings == nil {
			return errors.New("steam reset not configured")
		}
		return resetSteamSettings()
	}
	path, err := platformSettingsJSONPath(platformKey)
	if err != nil {
		return err
	}
	def := DefaultPlatformSettings()
	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}
