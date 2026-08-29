package steam

import (
	"reflect"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestBuildSteamArgsCarriesConfiguredOfflineFlagThroughSwitchLaunch(t *testing.T) {
	settings := Settings{
		PlatformSettings: platform.PlatformSettings{
			LaunchArguments: "-silent -offline",
		},
	}

	got := buildSteamArgs(settings, []string{"-applaunch", "730"})
	want := []string{"-silent", "-offline", "-applaunch", "730"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSteamArgs() = %v, want %v", got, want)
	}
	if !steamOfflineModeEnabled(settings) {
		t.Fatal("configured -offline argument should enable offline account state")
	}
	settings.LaunchArguments = "-silent"
	if steamOfflineModeEnabled(settings) {
		t.Fatal("settings without -offline should disable offline account state")
	}
}
