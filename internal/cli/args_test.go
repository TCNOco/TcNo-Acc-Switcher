package cli

import (
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
