package app

import (
	"testing"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestMainWindowOptionsExposeBrowserTools(t *testing.T) {
	opts := mainWindowOptions(platform.AppSettings{}, cli.Parsed{})

	if !opts.DevToolsEnabled {
		t.Fatal("DevToolsEnabled = false, want true")
	}
	if opts.DefaultContextMenuDisabled {
		t.Fatal("DefaultContextMenuDisabled = true, want false")
	}
	for _, accelerator := range []string{"Ctrl+Shift+I", "F11"} {
		if opts.KeyBindings[accelerator] == nil {
			t.Fatalf("KeyBindings missing %q", accelerator)
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
