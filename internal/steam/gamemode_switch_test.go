package steam

import (
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/cli"
)

func TestShouldHandOffSwitch(t *testing.T) {
	cases := []struct {
		name      string
		gamescope bool
		isHelper  bool
		want      bool
	}{
		{name: "desktop", want: false},
		{name: "Game Mode", gamescope: true, want: true},
		// Without this the helper hands the swap to another helper, forever.
		{name: "the helper itself", gamescope: true, isHelper: true, want: false},
		{name: "helper somehow on a desktop", isHelper: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldHandOffSwitch(tc.gamescope, tc.isHelper); got != tc.want {
				t.Fatalf("shouldHandOffSwitch(%v, %v) = %v, want %v", tc.gamescope, tc.isHelper, got, tc.want)
			}
		})
	}
}

func TestStartsSteamAfterSwap(t *testing.T) {
	// In Game Mode the session unit restarts Steam; a second one racing it is
	// worse than none.
	for _, tc := range []struct {
		autoStart, gameMode, want bool
	}{
		{autoStart: true, gameMode: false, want: true},
		{autoStart: true, gameMode: true, want: false},
		{autoStart: false, gameMode: false, want: false},
		{autoStart: false, gameMode: true, want: false},
	} {
		if got := startsSteamAfterSwap(tc.autoStart, tc.gameMode); got != tc.want {
			t.Fatalf("startsSteamAfterSwap(%v, %v) = %v, want %v", tc.autoStart, tc.gameMode, got, tc.want)
		}
	}
}

// The helper only works if it can read back the argv it was given, so this
// round-trips through the parser the helper actually runs.
func TestSwitchHelperArgsRoundTripThroughTheParser(t *testing.T) {
	cases := []struct {
		name         string
		steamID64    string
		personaState int
		wantKind     cli.Kind
		wantID       string
		wantPersona  int
	}{
		{
			name: "plain switch", steamID64: "76561198000000001", personaState: -1,
			wantKind: cli.KindSwapSteam, wantID: "76561198000000001", wantPersona: -1,
		},
		{
			name: "switch with a persona state", steamID64: "76561198000000001", personaState: 7,
			wantKind: cli.KindSwapSteam, wantID: "76561198000000001", wantPersona: 7,
		},
		{
			name: "persona state 0 is not the same as none", steamID64: "76561198000000001", personaState: 0,
			wantKind: cli.KindSwapSteam, wantID: "76561198000000001", wantPersona: 0,
		},
		{
			name: "add new signs the current account out", steamID64: "", personaState: -1,
			wantKind: cli.KindLogout,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := switchHelperArgs(tc.steamID64, tc.personaState)
			if args[len(args)-1] != gameModeSwitchFlag {
				t.Fatalf("args = %v, want %s last", args, gameModeSwitchFlag)
			}

			parsed, err := cli.Parse(args, nil)
			if err != nil {
				t.Fatalf("Parse(%v): %v", args, err)
			}
			if !parsed.GameModeSwitch {
				t.Error("GameModeSwitch is false; the helper would hand the swap off again")
			}
			if parsed.Kind != tc.wantKind {
				t.Fatalf("Kind = %v, want %v", parsed.Kind, tc.wantKind)
			}
			if tc.wantKind != cli.KindSwapSteam {
				return
			}
			if parsed.SteamID64 != tc.wantID {
				t.Errorf("SteamID64 = %q, want %q", parsed.SteamID64, tc.wantID)
			}
			if parsed.PersonaState != tc.wantPersona {
				t.Errorf("PersonaState = %d, want %d", parsed.PersonaState, tc.wantPersona)
			}
		})
	}
}

func TestGameModeSwitchFlagIsNotDocumented(t *testing.T) {
	// Internal, like --clean-legacy-install: the app sets it, never a user.
	if strings.Contains(cli.HelpText(), gameModeSwitchFlag) {
		t.Fatalf("%s appears in HelpText", gameModeSwitchFlag)
	}
}
