package steam

import (
	"log/slog"
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

// InGamescopeSession reports that this is Steam's Game Mode - a gamescope
// session on a handheld - rather than a desktop with Steam running on it.
//
// The difference decides whether leaving Steam's process tree is the right move
// or the wrong one. On a desktop the window belongs to the user's compositor and
// survives on its own. Under gamescope there is no other compositor: gamescope
// draws the window of the process Steam launched as the game, so a process that
// has moved out of that tree renders into nothing. Measured on an ROG Ally: the
// relaunched instance came up healthy on DISPLAY=:1 and was never shown.
func InGamescopeSession() bool { return inGamescopeSession(os.Getenv) }

// inGamescopeSession takes its environment so the shapes can be checked off-device.
//
// The values are the ones a Game Mode launch on Bazzite 43 actually carries:
// GAMESCOPE_WAYLAND_DISPLAY=gamescope-0, XDG_CURRENT_DESKTOP=gamescope,
// DESKTOP_SESSION=gamescope-session.
//
// SteamGamepadUI is deliberately not one of them. Big Picture sets it on an
// ordinary desktop too, and there the breakaway is right.
func inGamescopeSession(env func(string) string) bool {
	if strings.TrimSpace(env("GAMESCOPE_WAYLAND_DISPLAY")) != "" {
		return true
	}
	for _, key := range []string{"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION"} {
		if strings.Contains(strings.ToLower(env(key)), "gamescope") {
			return true
		}
	}
	return false
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
		slog.Info("startup: not a Steam launch, staying where we are", steamLaunchEnvAttrs()...)
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	slog.Info("startup: Steam started us, moving out of its process tree", steamLaunchEnvAttrs()...)
	if err := relaunchOutsideSteam(exe, os.Args[1:]); err != nil {
		return false, err
	}
	slog.Info("startup: relaunched outside Steam's process tree, this process is done")
	return true, nil
}

// steamLaunchEnvKeys are the variables that decide what happens at startup, or
// explain it afterwards. A fixed list rather than the whole environment, which
// carries tokens.
var steamLaunchEnvKeys = []string{
	"SteamClientLaunch",
	"SteamEnv",
	"SteamAppId",
	"SteamGameId",
	"SteamDeck",
	"SteamGamepadUI",
	"GAMESCOPE_WAYLAND_DISPLAY",
	"XDG_CURRENT_DESKTOP",
	"XDG_SESSION_TYPE",
	"DESKTOP_SESSION",
	"WAYLAND_DISPLAY",
	"DISPLAY",
}

// steamLaunchEnvAttrs records what the process was handed. Without it a log
// showing a healthy startup and a log showing a startup that was about to hand
// over and exit read exactly the same, which is how "it launches but no window
// appears" stayed ambiguous across three rounds of testing on a handheld.
func steamLaunchEnvAttrs() []any {
	attrs := make([]any, 0, len(steamLaunchEnvKeys)*2)
	for _, key := range steamLaunchEnvKeys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			attrs = append(attrs, key, value)
		}
	}
	return attrs
}
