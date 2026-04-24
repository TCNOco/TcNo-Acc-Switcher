package cli

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestParsePassthroughLaunchArgs(t *testing.T) {
	idx := &PlatformIndex{
		Names: map[string]string{"steam": "Steam"},
	}
	argv := []string{"+s:76561198064588130", "-dev", "-x", "-y"}
	p, err := Parse(argv, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSwapSteam {
		t.Fatalf("kind: got %v want KindSwapSteam", p.Kind)
	}
	if p.SteamID64 != "76561198064588130" {
		t.Fatalf("steam id: got %q", p.SteamID64)
	}
	want := []string{"-dev", "-x", "-y"}
	if !reflect.DeepEqual(p.PassthroughLaunchArgs, want) {
		t.Fatalf("passthrough: got %#v want %#v", p.PassthroughLaunchArgs, want)
	}
}

func TestParseRunAppIDSteam(t *testing.T) {
	idx := &PlatformIndex{Names: map[string]string{"steam": "Steam"}}
	p, err := Parse([]string{"+s:76561198064588130", "--run-appid=945360"}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSwapSteam || p.RunAppID != "945360" || p.RunShortcutFile != "" {
		t.Fatalf("got %#v", p)
	}
}

func TestParseRunAppIDRequiresSteam(t *testing.T) {
	idx := &PlatformIndex{
		Names:            map[string]string{"steam": "Steam", "epic": "Epic Games"},
		FirstIdentifier:  map[string]string{"e": "Epic Games"},
	}
	_, err := Parse([]string{"+e:someid", "--run-appid=123"}, idx)
	if err == nil || !strings.Contains(err.Error(), "--run-appid requires") {
		t.Fatalf("want error, got %v", err)
	}
}

func TestParseRunShortcutWithSteam(t *testing.T) {
	idx := &PlatformIndex{Names: map[string]string{"steam": "Steam"}}
	enc := url.QueryEscape("My Game.lnk")
	p, err := Parse([]string{"+s:76561198064588130", "--run-shortcut=" + enc}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindSwapSteam || p.RunShortcutFile != "My Game.lnk" || p.RunAppID != "" {
		t.Fatalf("got %#v", p)
	}
}

func TestParseRunShortcutMutuallyExclusive(t *testing.T) {
	idx := &PlatformIndex{Names: map[string]string{"steam": "Steam"}}
	_, err := Parse([]string{"+s:76561198064588130", "--run-appid=1", "--run-shortcut=x.lnk"}, idx)
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
