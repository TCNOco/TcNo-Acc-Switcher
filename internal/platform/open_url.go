package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"TcNo-Acc-Switcher/internal/winutil"
)

// openWithDefaultHandler hands a URI to whatever the desktop has registered for
// its scheme.
//
// The Windows arm is `cmd.exe /c start` with an empty title argument: start
// treats a lone quoted argument as the window title, so dropping it would open a
// console instead of the target. On Linux xdg-open resolves the scheme through
// the desktop's handler registry, so steam:// reaches whichever Steam is
// installed - native, Flatpak or Snap - without this code knowing which.
func openWithDefaultHandler(uri string) error {
	switch runtime.GOOS {
	case "windows":
		return winutil.Start("cmd.exe", []string{"/c", "start", "", uri}, winutil.StartOpts{HideWindow: true})
	case "darwin":
		return exec.Command("open", uri).Start()
	default:
		return exec.Command("xdg-open", uri).Start()
	}
}

// OpenURL opens a http/https URL in the user's default browser.
//
// The scheme is checked because these URLs are built from app IDs and profile
// data: every handler here will as happily launch a local path or another
// scheme, so a malformed id must not turn into an arbitrary launch.
func OpenURL(url string) error {
	url = strings.TrimSpace(url)
	lower := strings.ToLower(url)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return fmt.Errorf("refusing to open non-web URL: %q", url)
	}
	return openWithDefaultHandler(url)
}

// OpenSteamURI hands a steam: URI to the installed Steam client.
//
// Separate from [OpenURL] so each keeps a scheme check that means something:
// this one must not be handed a web address and that one must not launch a game.
func OpenSteamURI(uri string) error {
	uri = strings.TrimSpace(uri)
	if !strings.HasPrefix(strings.ToLower(uri), "steam://") {
		return fmt.Errorf("refusing to open non-Steam URI: %q", uri)
	}
	return openWithDefaultHandler(uri)
}
