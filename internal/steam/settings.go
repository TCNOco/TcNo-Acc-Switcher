package steam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

const settingsFileName = "SteamSettings.json"

// Settings mirrors legacy SteamSettings.json (C#) field names for compatibility where applicable.
// Shared fields are embedded from [platform.PlatformSettings].
type Settings struct {
	platform.PlatformSettings

	FolderPath string `json:"FolderPath"`

	SteamShowSteamID     bool `json:"Steam_ShowSteamID"`
	SteamShowVAC         bool `json:"Steam_ShowVAC"`
	SteamShowLimited     bool `json:"Steam_ShowLimited"`
	SteamShowLastLogin   bool `json:"Steam_ShowLastLogin"`
	SteamShowAccUsername bool `json:"Steam_ShowAccUsername"`
	SteamTrayAccountName bool `json:"Steam_TrayAccountName"`

	SteamImageExpiryTime int `json:"Steam_ImageExpiryTime"`
	SteamOverrideState   int `json:"Steam_OverrideState"`

	ShortcutsJSON map[string]string `json:"ShortcutsJson"`

	SteamWebAPIKey string `json:"SteamWebApiKey"`

	StartSilent       bool `json:"StartSilent"`
	OldUi             bool `json:"OldUi"`
	ShowSteamSwitcher bool `json:"ShowSteamSwitcher"`
	CollectInfo       bool `json:"CollectInfo"`
}

func defaultSettings() Settings {
	ps := platform.DefaultPlatformSettings()
	return Settings{
		PlatformSettings:     ps,
		FolderPath:           `C:\Program Files (x86)\Steam\`,
		SteamShowVAC:         true,
		SteamShowLimited:     true,
		SteamShowLastLogin:   true,
		SteamShowAccUsername: true,
		SteamShowSteamID:     false,
		SteamImageExpiryTime: 7,
		SteamOverrideState:   -1,
		ShortcutsJSON:        map[string]string{},
		CollectInfo:          true,
	}
}

func settingsPath() (string, error) {
	dir, err := paths.SettingsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, settingsFileName), nil
}

// ResetToDefaults overwrites SteamSettings.json with defaults (wired from [platform.SetSteamReset]).
func ResetToDefaults() error {
	return SaveSettings(defaultSettings())
}

// LoadSettings reads Settings/SteamSettings.json.
func LoadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return Settings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return Settings{}, err
	}

	// Migrate legacy JSON keys into embedded PlatformSettings before unmarshalling.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return defaultSettings(), err
	}
	if _, has := raw["RunAsAdmin"]; !has {
		if v, ok := raw["Steam_Admin"]; ok {
			raw["RunAsAdmin"] = v
		}
		delete(raw, "Steam_Admin")
	}
	if _, has := raw["TrayAccNumber"]; !has {
		if v, ok := raw["Steam_TrayAccNumber"]; ok {
			raw["TrayAccNumber"] = v
		}
		delete(raw, "Steam_TrayAccNumber")
	}
	data2, err := json.Marshal(raw)
	if err != nil {
		return Settings{}, err
	}

	var s Settings
	if err := json.Unmarshal(data2, &s); err != nil {
		return defaultSettings(), err
	}
	s.FolderPath = NormalizeFolderPath(s.FolderPath)
	if s.ShortcutsJSON == nil {
		s.ShortcutsJSON = map[string]string{}
	}
	if s.AccountNotes == nil {
		s.AccountNotes = map[string]string{}
	}
	if s.SteamImageExpiryTime <= 0 {
		s.SteamImageExpiryTime = 7
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
	return s, nil
}

// SaveSettings writes Settings/SteamSettings.json.
func SaveSettings(s Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if s.ShortcutsJSON == nil {
		s.ShortcutsJSON = map[string]string{}
	}
	if s.AccountNotes == nil {
		s.AccountNotes = map[string]string{}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

// NormalizeFolderPath strips a trailing steam.exe and ensures directory form.
func NormalizeFolderPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(p), ".exe") {
		p = filepath.Dir(p)
	}
	p = filepath.Clean(p)
	if len(p) >= 2 && p[1] == ':' && !strings.HasSuffix(p, `\`) {
		// keep as clean path
	}
	return p
}

// PlatformKey is the folder name under img/profiles.
const PlatformKey = "Steam"
const ProfileFolderSlug = "steam"
