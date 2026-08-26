package platform

import (
	"os"
	"strings"
)

// InGamescopeSession reports that this is a gamescope session - Steam's Game
// Mode on a handheld - rather than a desktop with Steam running on it: one app
// on screen, no other compositor, no window management and no tray.
func InGamescopeSession() bool { return inGamescopeSession(os.Getenv) }

// GamescopeSessionEnv carries the verdict to a process this app starts through
// the user's systemd manager. Such a process inherits the manager's environment,
// which has none of the session variables the verdict is read from, so it has to
// be decided here and passed on explicitly.
const GamescopeSessionEnv = "TCNO_GAMESCOPE_SESSION"

// inGamescopeSession takes its environment so the shapes can be checked
// off-device. A Game Mode launch carries GAMESCOPE_WAYLAND_DISPLAY=gamescope-0,
// XDG_CURRENT_DESKTOP=gamescope and DESKTOP_SESSION=gamescope-session.
//
// SteamGamepadUI is deliberately not one of them. Big Picture sets it on an
// ordinary desktop too, where the window belongs to a real compositor and none
// of this applies.
func inGamescopeSession(env func(string) string) bool {
	if strings.TrimSpace(env(GamescopeSessionEnv)) != "" {
		return true
	}
	if strings.TrimSpace(env("GAMESCOPE_WAYLAND_DISPLAY")) != "" {
		return true
	}
	// XDG_SESSION_DESKTOP is set by some distributions' session files and not the
	// others.
	for _, key := range []string{"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION", "XDG_SESSION_DESKTOP"} {
		if strings.Contains(strings.ToLower(env(key)), "gamescope") {
			return true
		}
	}
	return false
}
