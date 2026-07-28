package steamguard

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestConfirmationsWindowOptionsAreHardened(t *testing.T) {
	options := confirmationsWindowOptions("hardened_account")
	if options.Name != confirmationsWindowName || options.URL != "/#/steam/confirmations" {
		t.Fatalf("identity = %q, %q", options.Name, options.URL)
	}
	if options.EnableFileDrop || options.DevToolsEnabled || !options.DefaultContextMenuDisabled || !options.ContentProtectionEnabled {
		t.Fatalf("unsafe window options = %#v", options)
	}
	for _, permission := range []application.PermissionType{
		application.PermissionCamera,
		application.PermissionMicrophone,
		application.PermissionGeolocation,
		application.PermissionNotifications,
		application.PermissionClipboardRead,
	} {
		if options.Permissions[permission] != application.PermissionDeny {
			t.Fatalf("permission %v = %v", permission, options.Permissions[permission])
		}
	}
}

func TestConfirmationsWindowTitleUsesAccountUsername(t *testing.T) {
	if got := confirmationsWindowTitle("some_user"); got != "some_user - Confirmations" {
		t.Fatalf("title = %q", got)
	}
	if got := confirmationsWindowTitle("  spaced_user\t"); got != "spaced_user - Confirmations" {
		t.Fatalf("trimmed title = %q", got)
	}
	for _, blank := range []string{"", "   "} {
		if got := confirmationsWindowTitle(blank); got != "Steam Guard Confirmations" {
			t.Fatalf("fallback title for %q = %q", blank, got)
		}
	}
	if options := confirmationsWindowOptions("some_user"); options.Title != "some_user - Confirmations" {
		t.Fatalf("options title = %q", options.Title)
	}
}

// The list scrolls, so the window opens taller than its minimum to show several
// rows while still shrinking to a size that fits the header and action bar.
func TestConfirmationsWindowOptionsOpenTallerThanTheirMinimum(t *testing.T) {
	options := confirmationsWindowOptions("some_user")
	if options.Width != 490 || options.Height != 700 {
		t.Fatalf("size = %dx%d", options.Width, options.Height)
	}
	if options.MinWidth != 490 || options.MinHeight != 455 {
		t.Fatalf("min size = %dx%d", options.MinWidth, options.MinHeight)
	}
}
