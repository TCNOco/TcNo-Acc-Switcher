package steam

import "testing"

func TestLaunchedBySteam(t *testing.T) {
	cases := []struct {
		name         string
		clientLaunch string
		brokeAway    string
		want         bool
	}{
		{name: "started from a desktop or shell", want: false},
		{name: "started by the Steam client", clientLaunch: "1", want: true},
		// Without this the Windows relaunch loops: the child inherits our whole
		// environment, SteamClientLaunch included, and relaunches itself forever.
		{name: "already moved out of Steam's tree", clientLaunch: "1", brokeAway: "1", want: false},
		{name: "marker set with no Steam launch", brokeAway: "1", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(steamClientLaunchEnv, tc.clientLaunch)
			t.Setenv(brokeAwayEnv, tc.brokeAway)
			if got := LaunchedBySteam(); got != tc.want {
				t.Fatalf("LaunchedBySteam() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBreakAwayDoesNothingWhenSteamDidNotStartUs(t *testing.T) {
	t.Setenv(steamClientLaunchEnv, "")
	t.Setenv(brokeAwayEnv, "")
	brokeAway, err := BreakAwayFromSteamLaunch()
	if brokeAway || err != nil {
		t.Fatalf("BreakAwayFromSteamLaunch() = %v, %v; want false, nil", brokeAway, err)
	}
}

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
			// The relaunched instance keeps the session variables but loses
			// Steam's, and still has to be recognised as Game Mode.
			name: "same session, no Steam variables",
			env: map[string]string{
				"XDG_CURRENT_DESKTOP": "gamescope",
				"DESKTOP_SESSION":     "gamescope-session",
				"DISPLAY":             ":1",
			},
			want: true,
		},
		{
			// Big Picture on a desktop sets SteamGamepadUI as well, and there
			// leaving Steam's process tree is the correct behaviour.
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
			got := inGamescopeSession(func(k string) string { return tc.env[k] })
			if got != tc.want {
				t.Fatalf("inGamescopeSession() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBreakAwayStaysInSteamsTreeUnderGamescope(t *testing.T) {
	// The assertion is really "it did not relaunch": if the gate regressed, this
	// test binary would spawn a copy of itself.
	t.Setenv(brokeAwayEnv, "")
	t.Setenv(steamClientLaunchEnv, "1")
	t.Setenv("XDG_CURRENT_DESKTOP", "gamescope")

	brokeAway, err := BreakAwayFromSteamLaunch()
	if brokeAway || err != nil {
		t.Fatalf("BreakAwayFromSteamLaunch() = %v, %v; want false, nil under gamescope", brokeAway, err)
	}
}
