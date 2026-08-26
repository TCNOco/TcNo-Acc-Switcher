//go:build windows

package steam

import (
	"os"

	"TcNo-Acc-Switcher/internal/winutil"
)

// relaunchOutsideSteam starts the app again through the launch seam that records
// the shell as the creator rather than us: with no creator link back to
// steam.exe, the `taskkill /T` the switcher runs to close Steam cannot reach the
// switcher.
func relaunchOutsideSteam(exe string, args []string) error {
	// CreateProcess is given no environment block, so the child gets ours - which
	// is how it learns not to do this a second time.
	if err := os.Setenv(brokeAwayEnv, "1"); err != nil {
		return err
	}
	return winutil.Start(exe, args, winutil.StartOpts{})
}
