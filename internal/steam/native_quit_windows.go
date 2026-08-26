//go:build windows

package steam

import (
	"fmt"
	"os"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// defaultSteamQuitArgs is the fallback for a catalog that predates Extras.QuitArgs.
var defaultSteamQuitArgs = []string{"-shutdown"}

// steamNativeQuit runs "steam.exe -shutdown".
//
// Steam treats WM_CLOSE on its main window as minimise-to-tray, so the generic graceful path can
// never make it exit: it expires its whole deadline and then force-kills, on every switch. The
// shutdown argument drains steam.exe and its steamwebhelper children instead.
func steamNativeQuit(root string) func() error {
	root = strings.TrimSpace(root)
	if root == "" {
		root = resolveSteamRootQuiet()
	}
	if root == "" {
		return nil
	}
	exe := SteamExePath(root)
	if st, err := os.Stat(exe); err != nil || st.IsDir() {
		return nil
	}
	args := descriptorQuitArgs()
	if len(args) == 0 {
		// A catalog too old to carry QuitArgs, or one a user has trimmed, would otherwise
		// put every Steam switch back on the graceful window that never works. Opting out
		// of a native quit entirely is done through the closing method: TaskKill and
		// Electron never invoke it, Combined and Close do.
		steamLog().Debug("steam descriptor has no QuitArgs; using built-in default", "args", defaultSteamQuitArgs)
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

// descriptorQuitArgs reads Extras.QuitArgs from the Steam descriptor, so a user can correct the
// argument without a rebuild. Steam resolves its install folder through its own code path rather
// than the basic flow, which is why it reads the field here and not through descriptorNativeQuit.
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
