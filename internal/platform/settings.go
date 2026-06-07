package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const settingsFileName = "TcNo-Acc-Switcher.settings.json"

// AppSettings is stored as indented JSON next to the executable for easy manual edits.
type AppSettings struct {
	Version int `json:"version"`

	Language string `json:"language"`

	Theme string `json:"theme,omitempty"`

	ThemeAccentPreset string `json:"themeAccentPreset,omitempty"`
	ThemeAccentCustom string `json:"themeAccentCustom,omitempty"`

	// AnimationsEnabled controls whether UI motion is active.
	// Stored without omitempty so false round-trips: omitted key plus normalize defaults would otherwise force true on load.
	AnimationsEnabled bool `json:"animationsEnabled"`

	PlatformOrder []string `json:"platformOrder"`

	DisabledPlatforms []string `json:"disabledPlatforms"`

	PlatformExePaths map[string]string `json:"platformExePaths,omitempty"`

	PlatformsJSONPath string `json:"platformsJsonPath,omitempty"`

	// ProtocolEnabled registers the tcno:// URL scheme on Windows when true.
	ProtocolEnabled bool `json:"protocolEnabled,omitempty"`

	// OfflineMode blocks outbound HTTP (avatars, Steam APIs, etc.) when true.
	OfflineMode bool `json:"offlineMode,omitempty"`

	// DiscordRpc enables Discord rich presence integration.
	// Stored without omitempty so false round-trips: omitted key plus normalize defaults would otherwise force true on load.
	DiscordRpc bool `json:"discordRpc"`

	// DiscordRpcShare controls whether total switch count is shown in Discord status.
	DiscordRpcShare bool `json:"discordRpcShare,omitempty"`

	// ExitToTray keeps the app running when the main window is closed; the window is hidden instead.
	ExitToTray bool `json:"exitToTray,omitempty"`

	// MinimizeOnSwitch hides the main window after a successful account switch (when launch succeeds if AutoStart is on).
	MinimizeOnSwitch bool `json:"minimizeOnSwitch,omitempty"`

	// StartTrayWithWindows registers Windows startup with the -tray flag (Tray-only launch).
	StartTrayWithWindows bool `json:"startTrayWithWindows,omitempty"`

	// StartProgramCentered places the main window in the center of the screen when the app opens.
	StartProgramCentered bool `json:"startProgramCentered,omitempty"`

	// StatsEnabled toggles local anonymous statistics collection.
	StatsEnabled bool `json:"statsEnabled,omitempty"`

	// StatsShare toggles anonymous statistics submission.
	StatsShare bool `json:"statsShare,omitempty"`

	// AppBgImage is the filename (under wwwroot/backgrounds/) of the app-wide background image.
	AppBgImage string `json:"appBgImage,omitempty"`

	// AppBgOpacity is the opacity of the app-wide background (0.0–1.0). 0 means use default (0.6).
	AppBgOpacity float64 `json:"appBgOpacity,omitempty"`

	// AppBgBlur is the blur radius in px for the app-wide background. 0 means use default (4.0).
	AppBgBlur float64 `json:"appBgBlur,omitempty"`

	// ThemeBgOverride is true when the user has explicitly set or cleared the app background,
	// overriding any background image bundled with the active theme.
	ThemeBgOverride bool `json:"themeBgOverride,omitempty"`

	// PlatformBgs stores per-platform background image settings keyed by platform name.
	PlatformBgs map[string]PlatformBgSettings `json:"platformBgs,omitempty"`
}

// PlatformBgSettings holds background image configuration for a single platform.
type PlatformBgSettings struct {
	// Image is the filename (under wwwroot/backgrounds/) of the platform background.
	Image string `json:"image,omitempty"`
	// Opacity is the opacity (0.0–1.0). 0 means use default (0.6).
	Opacity float64 `json:"opacity,omitempty"`
	// Blur is the blur radius in px. 0 means use default (4.0).
	Blur float64 `json:"blur,omitempty"`
}

// AppBackgroundInfo is returned to the frontend with background image state.
type AppBackgroundInfo struct {
	HasImage        bool    `json:"hasImage"`
	ImageURL        string  `json:"imageUrl"`
	Opacity         float64 `json:"opacity"`
	Blur            float64 `json:"blur"`
	ThemeBgOverride bool    `json:"themeBgOverride"`
}

func defaultSettings() AppSettings {
	return AppSettings{
		Version:           1,
		Language:          "en-US",
		PlatformExePaths:  map[string]string{},
		PlatformOrder:     nil,
		DisabledPlatforms: nil,
		StatsEnabled:      true,
		StatsShare:        true,
		DiscordRpc:        true,
		DiscordRpcShare:   false,
		AnimationsEnabled: true,
	}
}

func normalizeAppSettingsDefaults(s *AppSettings, raw map[string]json.RawMessage) {
	if s == nil {
		return
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Language == "" {
		s.Language = "en-US"
	}
	if s.PlatformExePaths == nil {
		s.PlatformExePaths = map[string]string{}
	}
	if _, ok := raw["statsEnabled"]; !ok {
		s.StatsEnabled = true
	}
	if _, ok := raw["statsShare"]; !ok {
		s.StatsShare = true
	}
	if _, ok := raw["discordRpc"]; !ok {
		s.DiscordRpc = true
	}
	if _, ok := raw["animationsEnabled"]; !ok {
		s.AnimationsEnabled = true
	}
	if !s.DiscordRpc {
		s.DiscordRpcShare = false
	}
	if s.OfflineMode {
		s.DiscordRpc = false
		s.DiscordRpcShare = false
	}
}

func settingsPath(exeDir string) string {
	return filepath.Join(exeDir, settingsFileName)
}

// LoadAppSettings reads TcNo-Acc-Switcher.settings.json next to the executable.
func LoadAppSettings(exeDir string) (AppSettings, error) {
	return loadSettings(exeDir)
}

// SaveAppSettings writes TcNo-Acc-Switcher.settings.json atomically.
func SaveAppSettings(exeDir string, s AppSettings) error {
	return saveSettingsAtomic(exeDir, s)
}

var (
	settingsCache struct {
		mu       sync.RWMutex
		exeDir   string
		settings AppSettings
		loaded   bool
	}
)

func loadSettingsFromDisk(exeDir string) (AppSettings, error) {
	path := settingsPath(exeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return AppSettings{}, err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	var s AppSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return AppSettings{}, err
	}
	normalizeAppSettingsDefaults(&s, raw)
	return s, nil
}

func loadSettings(exeDir string) (AppSettings, error) {
	settingsCache.mu.RLock()
	if settingsCache.loaded && settingsCache.exeDir == exeDir {
		s := settingsCache.settings
		settingsCache.mu.RUnlock()
		return s, nil
	}
	settingsCache.mu.RUnlock()

	settingsCache.mu.Lock()
	defer settingsCache.mu.Unlock()

	if settingsCache.loaded && settingsCache.exeDir == exeDir {
		return settingsCache.settings, nil
	}

	s, err := loadSettingsFromDisk(exeDir)
	if err != nil {
		return s, err
	}
	settingsCache.exeDir = exeDir
	settingsCache.settings = s
	settingsCache.loaded = true
	return s, nil
}

func saveSettingsAtomic(exeDir string, s AppSettings) error {
	if s.PlatformExePaths == nil {
		s.PlatformExePaths = map[string]string{}
	}
	normalizeAppSettingsDefaults(&s, map[string]json.RawMessage{
		"animationsEnabled": {},
		"statsEnabled":      {},
		"statsShare":        {},
		"discordRpc":        {},
	})
	path := settingsPath(exeDir)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteBytes(path, data, 0o644); err != nil {
		return err
	}

	settingsCache.mu.Lock()
	settingsCache.exeDir = exeDir
	settingsCache.settings = s
	settingsCache.loaded = true
	settingsCache.mu.Unlock()

	return nil
}

func atomicWriteBytes(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if perm != 0 {
		if err := os.Chmod(tmpPath, perm); err != nil {
			cleanup()
			return err
		}
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

var (
	exeDirOnce sync.Once
	exeDirVal  string
	exeDirErr  error
)

// ResetPathSingletonsForTest resets all cached path singletons for the given exe dir.
// Call once per test, before any flow operations. Do not use t.Parallel().
func ResetPathSingletonsForTest(exeDir string) {
	exeDirVal = exeDir
	exeDirErr = nil
	exeDirOnce = sync.Once{}
	exeDirOnce.Do(func() {})
}

// ResolveExeDir returns the directory containing the running executable.
func ResolveExeDir() (string, error) {
	exeDirOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			exeDirErr = err
			return
		}
		exeDirVal = filepath.Dir(exe)
	})
	return exeDirVal, exeDirErr
}
