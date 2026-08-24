package steam

import (
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// nativeQuitOpts returns kill options that ask Steam to shut itself down the way
// its own tray "Exit" item does, instead of relying on window messages.
//
// root may be empty, in which case it is resolved from settings; if it cannot be
// resolved the options come back with no NativeQuit and the caller falls back to
// the generic path.
func nativeQuitOpts(root string) winutil.KillOpts {
	quit := steamNativeQuit(root)
	if quit == nil {
		return winutil.KillOpts{}
	}
	return winutil.KillOpts{NativeQuit: quit}
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
