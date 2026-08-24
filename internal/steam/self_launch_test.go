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
