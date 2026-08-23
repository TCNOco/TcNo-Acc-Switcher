package steam

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

// defaultSteamExeNameFor is what Steam calls its launcher on goos, for when the
// catalog cannot be read. Windows and Linux keep it inside the install root;
// macOS keeps the app bundle elsewhere entirely, which is why [SteamExePath]
// falls back to the catalog's absolute paths rather than joining this to a root.
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

// steamExeBaseName is the launcher's filename, taken from the catalog so the
// name lives in Platforms.json with every other platform's.
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

// steamExeFromCatalog is the first catalog path that exists on this machine.
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

// SteamExePath resolves the launcher to run for an already-resolved install root.
//
// The root wins when it holds the launcher, because a user who pointed the
// switcher at one of several Steam installs must get that one and not whichever
// the catalog lists first. Only when the root has no launcher in it does the
// catalog's own absolute path get a say - which is the normal case on macOS,
// where Steam.app lives in /Applications and only its data is under the root.
//
// The returned path is not guaranteed to exist: when nothing can be found, the
// path the caller expected comes back so the failure names something useful.
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
