package platform

import "testing"

func TestInGamescopeSession(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			// Captured from a Game Mode launch on an ROG Ally, Bazzite 43.
			name: "Game Mode on a handheld",
			env: map[string]string{
				"GAMESCOPE_WAYLAND_DISPLAY": "gamescope-0",
				"XDG_CURRENT_DESKTOP":       "gamescope",
				"DESKTOP_SESSION":           "gamescope-session",
				"XDG_SESSION_TYPE":          "x11",
				"DISPLAY":                   ":1",
				"SteamGamepadUI":            "1",
			},
			want: true,
		},
		{
			name: "session variables without gamescope's own",
			env:  map[string]string{"XDG_CURRENT_DESKTOP": "gamescope", "DISPLAY": ":1"},
			want: true,
		},
		{
			name: "only XDG_SESSION_DESKTOP names it",
			env:  map[string]string{"XDG_SESSION_DESKTOP": "gamescope-session-steam"},
			want: true,
		},
		{
			// Big Picture sets SteamGamepadUI on a desktop too, and there the
			// window belongs to a real compositor.
			name: "Big Picture on a KDE desktop",
			env: map[string]string{
				"XDG_CURRENT_DESKTOP": "KDE",
				"DESKTOP_SESSION":     "plasmawayland",
				"WAYLAND_DISPLAY":     "wayland-0",
				"SteamGamepadUI":      "1",
				"SteamClientLaunch":   "1",
			},
			want: false,
		},
		{name: "plain desktop", env: map[string]string{"XDG_CURRENT_DESKTOP": "KDE"}, want: false},
		{name: "nothing set", env: map[string]string{}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inGamescopeSession(func(k string) string { return tc.env[k] }); got != tc.want {
				t.Fatalf("inGamescopeSession() = %v, want %v", got, tc.want)
			}
		})
	}
}
