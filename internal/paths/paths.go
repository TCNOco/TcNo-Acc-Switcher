// Package paths resolves the app's data directory layout.
package paths

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

var (
	dataRootMu  sync.RWMutex
	dataRoot    string
	dataRootSet bool
)

// InitDataRoot sets the resolved user data directory. Call once at startup from platform.InitDataPaths.
func InitDataRoot(dir string) {
	dataRootMu.Lock()
	dataRoot = filepath.Clean(dir)
	dataRootSet = true
	dataRootMu.Unlock()
}

// DataRoot returns the resolved user data directory set by [InitDataRoot].
func DataRoot() (string, error) {
	dataRootMu.RLock()
	defer dataRootMu.RUnlock()
	if !dataRootSet || dataRoot == "" {
		return "", errors.New("data root not initialized")
	}
	return dataRoot, nil
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
	InitDataRoot(dataDir)

	loginCacheDirBase = filepath.Join(dataDir, "LoginCache")
	loginCacheDirErr = nil
	loginCacheDirOnce = sync.Once{}
	loginCacheDirOnce.Do(func() {})

	webviewCacheDir = filepath.Join(dataDir, "WebViewCache")
	webviewCacheDirErr = nil
	webviewCacheDirOnce = sync.Once{}
	webviewCacheDirOnce.Do(func() {})
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

var (
	webviewCacheDirOnce sync.Once
	webviewCacheDir     string
	webviewCacheDirErr  error
)

// WebViewCacheDir returns {DataRoot}/WebViewCache/ (WebView2 user-data folder on Windows).
func WebViewCacheDir() (string, error) {
	webviewCacheDirOnce.Do(func() {
		r, err := DataRoot()
		if err != nil {
			webviewCacheDirErr = err
			return
		}
		webviewCacheDir = filepath.Join(r, "WebViewCache")
	})
	return webviewCacheDir, webviewCacheDirErr
}
