package steam

import (
	"os"
	"strings"
)

// Steam does not merely start a non-Steam shortcut, it owns it. On Linux the
// command runs under Steam's `reaper`, and shutting Steam down takes the whole
// thing with it; on Windows the app is a descendant of steam.exe, which the
// switcher's own `taskkill /T` walks straight down.
//
// Either way the switcher dies exactly when it closes Steam - which is what
// switching an account does - so the entry in Steam's library is only worth
// having if the process leaves that tree before it does anything else.
//
// Measured on a Bazzite VM against a real Steam, in the order they were tried:
// launching directly dies; setsid dies, so the reaper is not signalling a
// process group; setsid --fork dies too, with the orphan reparented back onto
// the reaper rather than to init, which is PR_SET_CHILD_SUBREAPER. Nothing
// forked from inside that tree can get out of it. Being started by something
// outside it works, and on Linux that is the user's systemd manager.
const (
	// steamClientLaunchEnv is set to 1 by the Steam client for anything it
	// starts, on both Windows and Linux.
	steamClientLaunchEnv = "SteamClientLaunch"

	// brokeAwayEnv marks the relaunched process, so it does not relaunch itself
	// again. It is load-bearing on Windows, where the child inherits our whole
	// environment and would otherwise still see SteamClientLaunch.
	brokeAwayEnv = "TCNO_STEAM_BREAKAWAY"
)

// LaunchedBySteam reports that the Steam client started this process and that
// nothing has moved it out of Steam's process tree yet.
func LaunchedBySteam() bool {
	if strings.TrimSpace(os.Getenv(brokeAwayEnv)) != "" {
		return false
	}
	return strings.TrimSpace(os.Getenv(steamClientLaunchEnv)) != ""
}

// BreakAwayFromSteamLaunch restarts the app outside Steam's process tree when
// Steam is what started it, and reports whether it did. When it returns true the
// caller has been replaced and must exit without doing anything else - notably
// without taking the single-instance lock, which the replacement needs.
//
// An error means the app is still inside Steam's tree. That is worth saying out
// loud but not worth refusing to start over: everything except closing Steam
// still works.
func BreakAwayFromSteamLaunch() (bool, error) {
	if !LaunchedBySteam() {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	if err := relaunchOutsideSteam(exe, os.Args[1:]); err != nil {
		return false, err
	}
	return true, nil
}
