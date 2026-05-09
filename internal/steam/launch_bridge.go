package steam

import (
	"os"
	"path/filepath"

	"TcNo-Acc-Switcher/internal/platform"
)

// SaveFolderFromConfirmedExe writes SteamSettings FolderPath from a chosen steam.exe and removes PlatformExePaths["Steam"].
func SaveFolderFromConfirmedExe(exeFullPath string) error {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return err
	}
	st, err := LoadSettings()
	if err != nil {
		return err
	}
	st.FolderPath = NormalizeFolderPath(filepath.Dir(exeFullPath))
	if err := SaveSettings(st); err != nil {
		return err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return err
	}
	if app.PlatformExePaths != nil {
		delete(app.PlatformExePaths, platformName)
	}
	return platform.SaveAppSettings(exeDir, app)
}

// ResolveSteamExePath returns path to steam.exe from SteamSettings / defaults if it exists.
func ResolveSteamExePath() (string, bool) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return "", false
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return "", false
	}
	st, err := LoadSettings()
	if err != nil {
		return "", false
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return "", false
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil || root == "" {
		return "", false
	}
	p := filepath.Join(root, "steam.exe")
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, true
	}
	return "", false
}
