// Package paths resolves the app's data directory layout (next to exe for now).
package paths

import (
	"path/filepath"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/platform"
)

var (
	dataRootOnce sync.Once
	dataRoot     string
	dataRootErr  error
)

// DataRoot returns {ExeDir}/TcNo Account Switcher/
func DataRoot() (string, error) {
	dataRootOnce.Do(func() {
		exeDir, err := platform.ResolveExeDir()
		if err != nil {
			dataRootErr = err
			return
		}
		dataRoot = platform.UserDataDir(exeDir)
	})
	return dataRoot, dataRootErr
}

var (
	settingsDirOnce sync.Once
	settingsDir     string
	settingsDirErr  error
)

func SettingsDir() (string, error) {
	settingsDirOnce.Do(func() {
		r, err := DataRoot()
		if err != nil {
			settingsDirErr = err
			return
		}
		settingsDir = filepath.Join(r, "Settings")
	})
	return settingsDir, settingsDirErr
}

func SanitizePathSegment(name string) string {
	out := WindowsFileName(name, 0)
	if out == "" {
		return ""
	}
	return out
}

func windowsReservedFileStem(s string) bool {
	u := strings.ToUpper(strings.TrimSpace(s))
	switch u {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(u) == 4 {
		c := u[3]
		if c >= '0' && c <= '9' {
			if strings.HasPrefix(u, "COM") || strings.HasPrefix(u, "LPT") {
				return true
			}
		}
	}
	return false
}

var (
	loginCacheDirOnce sync.Once
	loginCacheDirBase string
	loginCacheDirErr  error
)

// ResetForTest resets all cached path singletons for the given exe dir.
func ResetForTest(dataDir string) {
	dataRoot = dataDir
	dataRootErr = nil
	dataRootOnce = sync.Once{}
	dataRootOnce.Do(func() {})

	loginCacheDirBase = filepath.Join(dataDir, "LoginCache")
	loginCacheDirErr = nil
	loginCacheDirOnce = sync.Once{}
	loginCacheDirOnce.Do(func() {})
}

func LoginCacheDir(platformKey string) (string, error) {
	loginCacheDirOnce.Do(func() {
		r, err := DataRoot()
		if err != nil {
			loginCacheDirErr = err
			return
		}
		loginCacheDirBase = filepath.Join(r, "LoginCache")
	})
	if loginCacheDirErr != nil {
		return "", loginCacheDirErr
	}
	seg := SanitizePathSegment(platformKey)
	if seg == "" {
		seg = "platform"
	}
	return filepath.Join(loginCacheDirBase, seg), nil
}

var (
	wwwrootDirOnce sync.Once
	wwwrootDir     string
	wwwrootDirErr  error
)

func WwwrootDir() (string, error) {
	wwwrootDirOnce.Do(func() {
		r, err := DataRoot()
		if err != nil {
			wwwrootDirErr = err
			return
		}
		wwwrootDir = filepath.Join(r, "wwwroot")
	})
	return wwwrootDir, wwwrootDirErr
}
