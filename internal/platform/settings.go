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

	PlatformOrder []string `json:"platformOrder"`

	DisabledPlatforms []string `json:"disabledPlatforms"`

	PlatformExePaths map[string]string `json:"platformExePaths,omitempty"`

	PlatformsJSONPath string `json:"platformsJsonPath,omitempty"`

	// ProtocolEnabled registers the tcno:// URL scheme on Windows when true.
	ProtocolEnabled bool `json:"protocolEnabled,omitempty"`

	// OfflineMode blocks outbound HTTP (avatars, Steam APIs, etc.) when true.
	OfflineMode bool `json:"offlineMode,omitempty"`

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

func loadSettings(exeDir string) (AppSettings, error) {
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
	return s, nil
}

func saveSettingsAtomic(exeDir string, s AppSettings) error {
	if s.PlatformExePaths == nil {
		s.PlatformExePaths = map[string]string{}
	}
	path := settingsPath(exeDir)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteBytes(path, data, 0o644)
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
