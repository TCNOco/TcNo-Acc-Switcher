// Package paths resolves the app's data directory layout (next to exe for now).
package paths

import (
	"path/filepath"

	"TcNo-Acc-Switcher/internal/platform"
)

const DataDirName = "TcNo Account Switcher"

// DataRoot returns {ExeDir}/TcNo Account Switcher/
func DataRoot() (string, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(exeDir, DataDirName), nil
}

func SettingsDir() (string, error) {
	r, err := DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "Settings"), nil
}

func LoginCacheDir(platformKey string) (string, error) {
	r, err := DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "LoginCache", platformKey), nil
}

func WwwrootDir() (string, error) {
	r, err := DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "wwwroot"), nil
}
