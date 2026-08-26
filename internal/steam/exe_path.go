package steam

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

// defaultSteamExeNameFor is the fallback for when the catalog cannot be read.
func defaultSteamExeNameFor(goos string) string {
	switch goos {
	case "windows":
		return "steam.exe"
	case "darwin":
		return "steam_osx"
	default:
		return "steam.sh"
	}
}

func steamExeBaseName() string {
	fallback := defaultSteamExeNameFor(runtime.GOOS)
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return fallback
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return fallback
	}
	entry, err := platform.ParsePlatformEntry(raw, platformName)
	if err != nil {
		return fallback
	}
	if exp := entry.ExeLocationDefault.FirstExpanded(); exp != "" {
		if base := filepath.Base(exp); base != "" && base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return fallback
}

func steamExeFromCatalog() string {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return ""
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return ""
	}
	entry, err := platform.ParsePlatformEntry(raw, platformName)
	if err != nil {
		return ""
	}
	return entry.ExeLocationDefault.FirstExistingExe()
}

// SteamExePath resolves the launcher for an already-resolved install root.
//
// The root wins when it holds the launcher, so a switcher pointed at one of
// several Steam installs gets that one. The catalog's absolute paths only get a
// say when the root has no launcher in it - the normal case on macOS, where
// Steam.app lives in /Applications and only its data sits under the root.
//
// The result is not guaranteed to exist; when nothing is found the expected path
// comes back so the failure names something useful.
func SteamExePath(root string) string {
	root = strings.TrimSpace(root)
	base := steamExeBaseName()
	if root != "" && base != "" {
		if p := filepath.Join(root, base); fileExists(p) {
			return p
		}
	}
	if found := steamExeFromCatalog(); found != "" {
		return found
	}
	if root == "" || base == "" {
		return ""
	}
	return filepath.Join(root, base)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
