package platform

import (
	"os"
	"strings"
)

// InGamescopeSession reports that this is a gamescope session - Steam's Game
// Mode on a handheld - rather than a desktop with Steam running on it.
//
// It lives here rather than with the Steam code because it is a fact about the
// session, not about Steam: there is one app on screen, no other compositor, no
// window management and no tray. What follows from it is the same whoever
// launched us - fill the screen, drop the window buttons that cannot work, and
// do not walk out of the process tree that owns the only window there is.
func InGamescopeSession() bool { return inGamescopeSession(os.Getenv) }

// inGamescopeSession takes its environment so the shapes can be checked
// off-device.
//
// The values are the ones a Game Mode launch on Bazzite 43 actually carries,
// read out of a real launch on an ROG Ally: GAMESCOPE_WAYLAND_DISPLAY=gamescope-0,
// XDG_CURRENT_DESKTOP=gamescope, DESKTOP_SESSION=gamescope-session.
//
// SteamGamepadUI is deliberately not one of them. Big Picture sets it on an
// ordinary desktop too, where the window belongs to a real compositor and none
// of this applies.
func inGamescopeSession(env func(string) string) bool {
	if strings.TrimSpace(env("GAMESCOPE_WAYLAND_DISPLAY")) != "" {
		return true
	}
	// XDG_SESSION_DESKTOP was not among the three the Ally carried, but session
	// files on other distributions set it and not always the others.
	for _, key := range []string{"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION", "XDG_SESSION_DESKTOP"} {
		if strings.Contains(strings.ToLower(env(key)), "gamescope") {
			return true
		}
	}
	return false
}
