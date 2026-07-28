package cli

import (
	"log/slog"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestParsePassthroughLaunchArgs(t *testing.T) {
	idx := &PlatformIndex{
		Names: map[string]string{"steam": "Steam"},
	}
	argv := []string{"+s:76561197960287930", "-dev", "-x", "-y"}
	p, err := Parse(argv, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSwapSteam {
		t.Fatalf("kind: got %v want KindSwapSteam", p.Kind)
	}
	if p.SteamID64 != "76561197960287930" {
		t.Fatalf("steam id: got %q", p.SteamID64)
	}
	want := []string{"-dev", "-x", "-y"}
	if !reflect.DeepEqual(p.PassthroughLaunchArgs, want) {
		t.Fatalf("passthrough: got %#v want %#v", p.PassthroughLaunchArgs, want)
	}
}

func TestParseRunAppIDSteam(t *testing.T) {
	idx := &PlatformIndex{Names: map[string]string{"steam": "Steam"}}
	p, err := Parse([]string{"+s:76561197960287930", "--run-appid=945360"}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSwapSteam || p.RunAppID != "945360" || p.RunShortcutFile != "" {
		t.Fatalf("got %#v", p)
	}
}

func TestParseRunAppIDRequiresSteam(t *testing.T) {
	idx := &PlatformIndex{
		Names:             map[string]string{"steam": "Steam", "epic": "Epic Games"},
		FirstIdentifier:   map[string]string{"e": "Epic Games"},
		IdentifierAliases: map[string]string{"e": "Epic Games", "epic": "Epic Games"},
	}
	_, err := Parse([]string{"+e:someid", "--run-appid=123"}, idx)
	if err == nil || !strings.Contains(err.Error(), "--run-appid requires") {
		t.Fatalf("want error, got %v", err)
	}
}

func TestParsePlusTokenAcceptsAnyIdentifierAlias(t *testing.T) {
	idx := &PlatformIndex{
		Names:             map[string]string{"steam": "Steam"},
		FirstIdentifier:   map[string]string{"s": "Steam"},
		IdentifierAliases: map[string]string{"s": "Steam", "steam": "Steam", "valve": "Steam"},
	}
	p, err := Parse([]string{"+valve:76561197960287930"}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSwapSteam || p.PlatformKey != "Steam" || p.SteamID64 != "76561197960287930" {
		t.Fatalf("got %#v", p)
	}
}

func TestParseOpenPageAcceptsIdentifierAlias(t *testing.T) {
	idx := &PlatformIndex{
		Names:             map[string]string{"steam": "Steam"},
		IdentifierAliases: map[string]string{"steam": "Steam", "valve": "Steam"},
	}
	p, err := Parse([]string{"--page=valve"}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindOpenPage || p.OpenPage != "Steam" {
		t.Fatalf("got %#v", p)
	}
}

func TestParseRunShortcutWithSteam(t *testing.T) {
	idx := &PlatformIndex{Names: map[string]string{"steam": "Steam"}}
	enc := url.QueryEscape("My Game.lnk")
	p, err := Parse([]string{"+s:76561197960287930", "--run-shortcut=" + enc}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSwapSteam || p.RunShortcutFile != "My Game.lnk" || p.RunAppID != "" {
		t.Fatalf("got %#v", p)
	}
}

func TestParseRunShortcutMutuallyExclusive(t *testing.T) {
	idx := &PlatformIndex{Names: map[string]string{"steam": "Steam"}}
	_, err := Parse([]string{"+s:76561197960287930", "--run-appid=1", "--run-shortcut=x.lnk"}, idx)
	if err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Fatalf("want error, got %v", err)
	}
}

func TestParseOpenPageFlag(t *testing.T) {
	idx := &PlatformIndex{
		Names: map[string]string{"steam": "Steam"},
	}
	p, err := Parse([]string{"--page=steam"}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindOpenPage || p.OpenPage != "Steam" {
		t.Fatalf("got %#v", p)
	}
	j := p.RouteJSONForOpenPage()
	if j == "" || !strings.Contains(j, "Steam") {
		t.Fatalf("route json: %q", j)
	}
}

func TestParseLogLevel(t *testing.T) {
	idx := &PlatformIndex{Names: map[string]string{"steam": "Steam"}}
	p, err := Parse([]string{"--log-level=warn", "Steam"}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if !p.LogLevelSet || p.LogLevel != slog.LevelWarn {
		t.Fatalf("log level: got set=%v level=%v", p.LogLevelSet, p.LogLevel)
	}
	if p.EffectiveSlogLevel() != slog.LevelWarn {
		t.Fatalf("effective: %v", p.EffectiveSlogLevel())
	}
}

func TestParseVerboseSetsDebug(t *testing.T) {
	p, err := Parse([]string{"-v"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Verbose != true || p.EffectiveSlogLevel() != slog.LevelDebug {
		t.Fatalf("got %#v effective=%v", p, p.EffectiveSlogLevel())
	}
}

func TestParseLogLevelOverridesVerbose(t *testing.T) {
	p, err := Parse([]string{"-v", "--log-level=error"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.EffectiveSlogLevel() != slog.LevelError {
		t.Fatalf("want error, got %v", p.EffectiveSlogLevel())
	}
}

func TestParseDuplicateLogLevel(t *testing.T) {
	_, err := Parse([]string{"--log-level=info", "--log-level=debug"}, nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate error, got %v", err)
	}
}

func TestParseAllowSteamGuardCapture(t *testing.T) {
	for _, arg := range []string{"--allow-steamguard-capture", "-allow-steamguard-capture"} {
		parsed, err := Parse([]string{arg}, nil)
		if err != nil || !parsed.AllowSteamGuardCapture {
			t.Fatalf("%s: parsed = %#v, err = %v", arg, parsed.AllowSteamGuardCapture, err)
		}
	}
	// Absent by default: capture stays blocked unless it is asked for.
	parsed, err := Parse([]string{"--verbose"}, nil)
	if err != nil || parsed.AllowSteamGuardCapture {
		t.Fatalf("parsed = %#v, err = %v", parsed.AllowSteamGuardCapture, err)
	}
}
