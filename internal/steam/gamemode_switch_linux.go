//go:build linux

package steam

import "TcNo-Acc-Switcher/internal/platform"

// startSwitchHelper runs the swap in a process the user's systemd manager owns,
// so closing Steam cannot take it down mid-write.
//
// No display variables are forwarded: the helper draws nothing, and the display
// it would inherit belongs to a session that is about to lose its client.
func startSwitchHelper(exe string, args []string) error {
	return systemdRunOutsideSteam(exe, args, map[string]string{
		brokeAwayEnv:                 "1",
		switchHelperEnv:              "1",
		platform.GamescopeSessionEnv: "1",
	}, []string{"HOME", "XDG_RUNTIME_DIR", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "LANG"})
}
