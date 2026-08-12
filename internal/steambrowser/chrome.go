package steambrowser

import (
	"fmt"
	"net/url"
	"strings"

	"TcNo-Acc-Switcher/internal/screenprivacy"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The chrome window: a Wails window on the application's own origin that draws
// the toolbar. The site itself is never loaded here — it lives in the content
// view underneath — so this window keeps the Wails runtime and bindings while
// the remote page has no access to either.

// windowName is unique per session, unlike the single confirmations window, so
// several accounts can be open at once and each is addressable.
func windowName(sessionID string) string {
	return "steam-browser:" + sessionID
}

// siteLabel names a destination in a window title. Not translated, unlike the
// UI that offers these: the title belongs to a window the operating system also
// labels, and the strings here are Steam's own product names.
func siteLabel(site Site) string {
	switch site {
	case SiteCommunity:
		return "Steam Community"
	case SiteChat:
		return "Steam Chat"
	case SiteGameDataCS2:
		return "Counter-Strike 2 Game Data"
	case SiteGameDataTF2:
		return "Team Fortress 2 Game Data"
	case SiteGameDataDota2:
		return "Dota 2 Game Data"
	default:
		return "Steam Store"
	}
}

func windowTitle(accountName string, site Site) string {
	label := siteLabel(site)
	accountName = strings.TrimSpace(accountName)
	if accountName == "" {
		return label
	}
	return accountName + " - " + label
}

func chromeWindowOptions(sessionID, title string) application.WebviewWindowOptions {
	options := application.WebviewWindowOptions{
		Name:      windowName(sessionID),
		Title:     title,
		URL:       "/#/steam/browser/" + sessionID,
		Width:     1100,
		Height:    800,
		MinWidth:  620,
		MinHeight: 400,
		Frameless: true,
		// This window hosts a remote page in a sibling view. It gets no file drop
		// from the OS, no capabilities, and no developer tools in production.
		EnableFileDrop:             false,
		DevToolsEnabled:            false,
		DefaultContextMenuDisabled: true,
		BackgroundColour:           application.NewRGB(27, 38, 54),
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionCamera:        application.PermissionDeny,
			application.PermissionMicrophone:    application.PermissionDeny,
			application.PermissionGeolocation:   application.PermissionDeny,
			application.PermissionNotifications: application.PermissionDeny,
			application.PermissionClipboardRead: application.PermissionDeny,
		},
	}
	// A Steam sign-in page is the last thing that should land in a recording.
	screenprivacy.Apply(&options)
	return options
}

// scaleForWindow converts device-independent pixels to physical ones. Wails
// reports window geometry in DIP while the content view is positioned in
// physical pixels, so on a scaled display the two disagree and the view would
// be placed wrong without this.
func scaleForWindow(window application.Window, value int) int {
	scale := 1.0
	if screen, err := window.GetScreen(); err == nil && screen != nil && screen.ScaleFactor > 0 {
		scale = float64(screen.ScaleFactor)
	}
	return int(float64(value)*scale + 0.5)
}

// normaliseTypedURL turns what a user typed or dropped into a URL worth
// navigating to.
//
// A bare host becomes https, never http, so a typed address is not silently
// downgraded. Anything that is not http or https is refused: a session window
// carries an account's cookies, and file: or javascript: has no business running
// in one.
func normaliseTypedURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("steambrowser: empty address")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("steambrowser: parse address: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", fmt.Errorf("steambrowser: refusing to open a %s address", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("steambrowser: address has no host")
	}
	return parsed.String(), nil
}
