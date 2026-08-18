package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// nativeQuitOpts returns kill options that ask Steam to shut itself down the way its own
// tray "Exit" item does, instead of relying on window messages.
//
// Steam treats WM_CLOSE on its main window as minimise-to-tray, so the generic graceful path
// can never make it exit: it expires its whole deadline and then force-kills, on every switch.
// "steam.exe -shutdown" drains steam.exe and its steamwebhelper children in ~1.6-2.5s.
//
// root may be empty, in which case it is resolved from settings; if it cannot be resolved the
// options come back with no NativeQuit and the caller falls back to the generic path.
func nativeQuitOpts(root string) winutil.KillOpts {
	quit := steamNativeQuit(root)
	if quit == nil {
		return winutil.KillOpts{}
	}
	return winutil.KillOpts{NativeQuit: quit}
}

func steamNativeQuit(root string) func() error {
	root = strings.TrimSpace(root)
	if root == "" {
		root = resolveSteamRootQuiet()
	}
	if root == "" {
		return nil
	}
	exe := filepath.Join(root, "steam.exe")
	if st, err := os.Stat(exe); err != nil || st.IsDir() {
		return nil
	}
	return func() error {
		// Through winutil.Start so the shutdown helper is spawned detached like every other
		// launch: it must not be able to take the switcher down with it, or vice versa.
		// Deliberately not elevated - a UAC prompt to close Steam would be worse than the
		// force-kill fallback this degrades to.
		if err := winutil.Start(exe, []string{"-shutdown"}, winutil.StartOpts{
			HideWindow: true,
			WorkingDir: root,
		}); err != nil {
			return fmt.Errorf("steam -shutdown: %w", err)
		}
		return nil
	}
}

// resolveSteamRootQuiet resolves the Steam install folder for callers that do not already
// hold one, returning "" rather than an error: a missing root only costs the fast close path.
func resolveSteamRootQuiet() string {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return ""
	}
	st, err := LoadSettings()
	if err != nil {
		return ""
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return ""
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return ""
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(root)
}
