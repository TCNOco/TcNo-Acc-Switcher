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
// defaultSteamQuitArgs is the fallback for a catalog that predates Extras.QuitArgs.
var defaultSteamQuitArgs = []string{"-shutdown"}

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
	args := descriptorQuitArgs()
	if len(args) == 0 {
		// A catalog too old to carry QuitArgs, or one a user has trimmed, would otherwise
		// silently put every Steam switch back on the 5s graceful window that never works.
		// Choosing no native quit at all is still available through the closing method:
		// TaskKill and Electron never reach this path.
		steamLog.Debug("steam descriptor has no QuitArgs; using built-in default", "args", defaultSteamQuitArgs)
		args = defaultSteamQuitArgs
	}
	return func() error {
		// Through winutil.Start so the shutdown helper is spawned detached like every other
		// launch: it must not be able to take the switcher down with it, or vice versa.
		// Deliberately not elevated - a UAC prompt to close Steam would be worse than the
		// force-kill fallback this degrades to.
		if err := winutil.Start(exe, args, winutil.StartOpts{
			HideWindow: true,
			WorkingDir: root,
		}); err != nil {
			return fmt.Errorf("steam quit %v: %w", args, err)
		}
		return nil
	}
}

// descriptorQuitArgs reads Extras.QuitArgs from the Steam descriptor so the argument lives in
// Platforms.json alongside every other platform's, and a user can correct it without a rebuild.
// Steam resolves its install folder through its own code path rather than the basic flow, which
// is why it reads the field here instead of going through descriptorNativeQuit.
func descriptorQuitArgs() []string {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return nil
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return nil
	}
	d, err := platform.ParseDescriptor(raw, PlatformKey)
	if err != nil {
		return nil
	}
	return platform.LaunchArgTokens(d.Extras.QuitArgs)
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
