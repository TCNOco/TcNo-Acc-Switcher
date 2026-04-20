package steam

import (
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

const platformName = "Steam"

// ResolveInstallFolder returns the Steam installation root (folder containing steam.exe).
// Order: SteamSettings.FolderPath → PlatformExePaths["Steam"] (exe dir) → ExeLocationDefault from Platforms.json.
func ResolveInstallFolder(exeDir string, s Settings, app platform.AppSettings, platformsJSON []byte) (string, error) {
	if fp := strings.TrimSpace(s.FolderPath); fp != "" {
		if st, err := os.Stat(filepath.Join(fp, "config", "loginusers.vdf")); err == nil && !st.IsDir() {
			return filepath.Clean(fp), nil
		}
		// path set but loginusers missing — still return folder for user to fix
		return filepath.Clean(fp), nil
	}

	if exe := strings.TrimSpace(app.PlatformExePaths[platformName]); exe != "" {
		dir := filepath.Dir(exe)
		if dir != "" && dir != "." {
			return filepath.Clean(dir), nil
		}
	}

	entry, err := platform.ParsePlatformEntry(platformsJSON, platformName)
	if err != nil {
		return "", err
	}
	def := strings.TrimSpace(entry.ExeLocationDefault)
	if def == "" {
		return "", nil
	}
	exp := platform.ExpandWindowsPath(def)
	if exp == "" {
		return "", nil
	}
	return filepath.Clean(filepath.Dir(exp)), nil
}

// LoginUsersPath returns .../config/loginusers.vdf under the Steam root.
func LoginUsersPath(steamRoot string) string {
	return filepath.Join(steamRoot, "config", "loginusers.vdf")
}
