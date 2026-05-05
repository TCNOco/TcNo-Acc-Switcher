package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/tidwall/gjson"
)

type GameShortcutEntry struct {
	FileName string `json:"fileName"`
	Pinned   bool   `json:"pinned"`
}

// UnmarshalJSON accepts alternate JSON key casing for FileName and Pinned.
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

type PlatformSettings struct {
	RunAsAdmin           bool                `json:"RunAsAdmin"`
	TrayAccNumber        int                 `json:"TrayAccNumber"`
	ForgetAccountEnabled bool                `json:"ForgetAccountEnabled"`
	ClosingMethod        string              `json:"ClosingMethod"`
	StartingMethod       string              `json:"StartingMethod"`
	AutoStart            bool                `json:"AutoStart"`
	ShowShortNotes       bool                `json:"ShowShortNotes"`
	ShowLastUsed         bool                `json:"ShowLastUsed"`
	AccountNotes         map[string]string   `json:"AccountNotes"`
	Shortcuts            []GameShortcutEntry `json:"Shortcuts,omitempty"`
	AlwaysSwapOnShortcut bool                `json:"AlwaysSwapOnShortcut,omitempty"`
	LaunchArguments      string              `json:"LaunchArguments,omitempty"`
	// ProfileImageExpiryDays is max age (days) for cached remote profile pictures (Platforms.json http(s) ProfilePicPath).
	ProfileImageExpiryDays int `json:"ProfileImageExpiryDays,omitempty"`
	// PullAccountImagesOnSwitch enables fetching profile images during account save/switch.
	PullAccountImagesOnSwitch bool `json:"PullAccountImagesOnSwitch,omitempty"`
}

func DefaultPlatformSettings() PlatformSettings {
	return PlatformSettings{
		TrayAccNumber:             3,
		ClosingMethod:             "Combined",
		StartingMethod:            "Default",
		AutoStart:                 true,
		ShowShortNotes:            true,
		ShowLastUsed:              true,
		AccountNotes:              map[string]string{},
		Shortcuts:                 []GameShortcutEntry{},
		ForgetAccountEnabled:      false,
		RunAsAdmin:                false,
		AlwaysSwapOnShortcut:      true,
		ProfileImageExpiryDays:    7,
		PullAccountImagesOnSwitch: true,
	}
}

func normalizeClosingMethodForOS(raw string) string {
	m := strings.TrimSpace(raw)
	if strings.EqualFold(m, string(winutil.ClosingCombined)) {
		return string(winutil.ClosingCombined)
	}
	if runtime.GOOS == "windows" {
		if strings.EqualFold(m, string(winutil.ClosingClose)) {
			return string(winutil.ClosingClose)
		}
		if strings.EqualFold(m, string(winutil.ClosingTaskKill)) {
			return string(winutil.ClosingTaskKill)
		}
		if strings.EqualFold(m, string(winutil.ClosingElectron)) {
			return string(winutil.ClosingElectron)
		}
	}
	return string(winutil.ClosingCombined)
}

func defaultClosingMethodForPlatform(platformKey string) string {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return string(winutil.ClosingCombined)
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return string(winutil.ClosingCombined)
	}
	raw, err := os.ReadFile(resolvePlatformsPath(exeDir, settings))
	if err != nil {
		return string(winutil.ClosingCombined)
	}
	d, err := ParseDescriptor(raw, platformKey)
	if err != nil {
		return string(winutil.ClosingCombined)
	}
	return normalizeClosingMethodForOS(d.Extras.ClosingMethod)
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

func LaunchArgTokens(line string) []string {
	return strings.Fields(strings.TrimSpace(line))
}

func HasLaunchArgToken(line, flag string) bool {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return false
	}
	for _, t := range LaunchArgTokens(line) {
		if strings.EqualFold(strings.TrimSpace(t), flag) {
			return true
		}
	}
	return false
}

func EnsureLaunchArg(line, flag string) string {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return strings.TrimSpace(line)
	}
	if HasLaunchArgToken(line, flag) {
		return strings.TrimSpace(line)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return flag
	}
	return line + " " + flag
}

func RemoveLaunchArgToken(line, flag string) string {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return strings.TrimSpace(line)
	}
	var out []string
	for _, t := range LaunchArgTokens(line) {
		if !strings.EqualFold(strings.TrimSpace(t), flag) {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

// LoadPlatformSettings reads Settings/<Platform>Settings.json (Steam: SteamSettings.json); only fields on PlatformSettings are unmarshaled.
func LoadPlatformSettings(platformKey string) (PlatformSettings, error) {
	defaultClosingMethod := defaultClosingMethodForPlatform(platformKey)
	path, err := platformSettingsJSONPath(platformKey)
	if err != nil {
		return PlatformSettings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s := DefaultPlatformSettings()
			s.ClosingMethod = defaultClosingMethod
			return s, nil
		}
		return PlatformSettings{}, err
	}
	var s PlatformSettings
	if err := json.Unmarshal(data, &s); err != nil {
		s := DefaultPlatformSettings()
		s.ClosingMethod = defaultClosingMethod
		return s, err
	}
	if gjson.GetBytes(data, "AlwaysSwapOnShortcut").Exists() {
		s.AlwaysSwapOnShortcut = gjson.GetBytes(data, "AlwaysSwapOnShortcut").Bool()
	} else {
		s.AlwaysSwapOnShortcut = true
	}
	if !gjson.GetBytes(data, "ShowLastUsed").Exists() {
		s.ShowLastUsed = true
	}
	if s.AccountNotes == nil {
		s.AccountNotes = map[string]string{}
	}
	// TrayAccNumber <= 0 disables tray MRU for this platform when explicitly set in JSON.
	if gjson.GetBytes(data, "TrayAccNumber").Exists() {
		// keep unmarshaled value (including 0)
	} else if s.TrayAccNumber <= 0 {
		s.TrayAccNumber = 3
	}
	s.ClosingMethod = normalizeClosingMethodForOS(s.ClosingMethod)
	if strings.TrimSpace(s.ClosingMethod) == "" {
		s.ClosingMethod = defaultClosingMethod
	}
	if strings.TrimSpace(s.StartingMethod) == "" {
		s.StartingMethod = "Default"
	}
	if s.Shortcuts == nil {
		s.Shortcuts = []GameShortcutEntry{}
	}
	if s.ProfileImageExpiryDays <= 0 {
		s.ProfileImageExpiryDays = 7
	}
	if !gjson.GetBytes(data, "PullAccountImagesOnSwitch").Exists() {
		s.PullAccountImagesOnSwitch = true
	}
	return s, nil
}

// SavePlatformSettings patches the JSON file without removing keys not present on PlatformSettings.
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

// resetPlatformJSONToDefaults resets common settings; Steam uses SetSteamReset for a full reset.
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
	def.ClosingMethod = defaultClosingMethodForPlatform(platformKey)
	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}
