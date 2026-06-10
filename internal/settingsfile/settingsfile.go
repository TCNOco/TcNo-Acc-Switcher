// Package settingsfile resolves TcNo-Acc-Switcher.settings.json locations without importing platform.
package settingsfile

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// FileName is the app-wide settings JSON filename.
	FileName = "TcNo-Acc-Switcher.settings.json"
	// UserDataDirName is the folder that holds per-user app data.
	UserDataDirName = "TcNo Account Switcher"
)

// PortableUserDataDir returns {exeDir}/TcNo Account Switcher/.
func PortableUserDataDir(exeDir string) string {
	return filepath.Join(filepath.Clean(exeDir), UserDataDirName)
}

// DefaultUserDataDir returns the default install location (%AppData%/TcNo Account Switcher).
func DefaultUserDataDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, UserDataDirName), nil
}

// DefaultSearchDirs returns the two default user data folders checked on launch (portable first).
func DefaultSearchDirs(exeDir string) []string {
	dirs := []string{PortableUserDataDir(exeDir)}
	if appData, err := DefaultUserDataDir(); err == nil {
		dirs = append(dirs, appData)
	}
	return dirs
}

// Discover finds an existing settings file. Portable user data is checked first, then AppData, then the exe dir (legacy/custom).
func Discover(exeDir string) (path string, ok bool) {
	exeDir = filepath.Clean(exeDir)
	for _, dir := range DefaultSearchDirs(exeDir) {
		if p, found := fileInDir(dir); found {
			return p, true
		}
	}
	return fileInDir(exeDir)
}

func fileInDir(dir string) (string, bool) {
	p := filepath.Join(filepath.Clean(dir), FileName)
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return "", false
	}
	return p, true
}

// IsDefaultUserDataDir reports whether dir is the portable or default AppData user data folder.
func IsDefaultUserDataDir(dir, exeDir string) bool {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" {
		return false
	}
	if strings.EqualFold(dir, PortableUserDataDir(exeDir)) {
		return true
	}
	if appData, err := DefaultUserDataDir(); err == nil {
		if strings.EqualFold(dir, filepath.Clean(appData)) {
			return true
		}
	}
	return false
}

// IsExeRootPath reports whether path is {exeDir}/TcNo-Acc-Switcher.settings.json.
func IsExeRootPath(exeDir, path string) bool {
	return strings.EqualFold(filepath.Clean(path), filepath.Join(filepath.Clean(exeDir), FileName))
}
