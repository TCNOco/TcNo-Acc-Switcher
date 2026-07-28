package app

import (
	"testing"

	"TcNo-Acc-Switcher/internal/buildmode"
	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestMainWindowOptionsApplyBuildBrowserPolicy(t *testing.T) {
	opts := mainWindowOptions(platform.AppSettings{}, cli.Parsed{})
	development := buildmode.IsDebugBuild()

	if opts.DevToolsEnabled != development {
		t.Fatalf("DevToolsEnabled = %v, want %v", opts.DevToolsEnabled, development)
	}
	if opts.DefaultContextMenuDisabled == development {
		t.Fatalf("DefaultContextMenuDisabled = %v for development=%v", opts.DefaultContextMenuDisabled, development)
	}
	if got := opts.KeyBindings["Ctrl+Shift+I"] != nil; got != development {
		t.Fatalf("Ctrl+Shift+I present = %v, want %v", got, development)
	}
	if opts.KeyBindings["F11"] == nil {
		t.Fatal("KeyBindings missing F11")
	}
}

func TestApplyWindowSecurityPolicyForRelease(t *testing.T) {
	opts := application.WebviewWindowOptions{
		DevToolsEnabled:            true,
		DefaultContextMenuDisabled: false,
		KeyBindings: map[string]func(application.Window){
			"Ctrl+Shift+I": func(application.Window) {},
			"F11":          func(application.Window) {},
		},
	}

	applyWindowSecurityPolicy(&opts, false)

	if opts.DevToolsEnabled {
		t.Fatal("DevToolsEnabled = true for release")
	}
	if !opts.DefaultContextMenuDisabled {
		t.Fatal("DefaultContextMenuDisabled = false for release")
	}
	if opts.KeyBindings["Ctrl+Shift+I"] != nil {
		t.Fatal("release key bindings expose DevTools")
	}
	if opts.KeyBindings["F11"] == nil {
		t.Fatal("release policy removed non-Debug key binding")
	}
	for _, permission := range []application.PermissionType{
		application.PermissionCamera,
		application.PermissionMicrophone,
		application.PermissionGeolocation,
		application.PermissionNotifications,
		application.PermissionClipboardRead,
	} {
		if opts.Permissions[permission] != application.PermissionDeny {
			t.Fatalf("permission %v is not denied", permission)
		}
	}
}

func TestMainWindowOptionsPreserveStartupPlacement(t *testing.T) {
	centered := mainWindowOptions(platform.AppSettings{StartProgramCentered: true}, cli.Parsed{})
	if centered.InitialPosition != application.WindowCentered {
		t.Fatalf("centered InitialPosition = %v", centered.InitialPosition)
	}

	hidden := mainWindowOptions(platform.AppSettings{}, cli.Parsed{StartInTray: true})
	if !hidden.Hidden {
		t.Fatal("Hidden = false, want true for tray startup")
	}
}

func TestGitHubUpdaterConfigUsesPrereleasePreference(t *testing.T) {
	stable := githubUpdaterConfig(platform.AppSettings{})
	if stable.Prerelease {
		t.Fatal("Prerelease = true, want false for an explicit opt-out")
	}

	preview := githubUpdaterConfig(platform.AppSettings{PrereleaseUpdates: true})
	if !preview.Prerelease {
		t.Fatal("Prerelease = false, want true when pre-release updates are enabled")
	}
}
