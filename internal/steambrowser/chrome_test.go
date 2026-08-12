package steambrowser

import "testing"

// A window opened from a link can land anywhere, so its caption has to describe
// where it actually went rather than the site it was linked from.
func TestWindowTitleNamesAnUntrustedHostRatherThanTheSite(t *testing.T) {
	steam, err := SiteCommunity.Destination(testSteamID)
	if err != nil {
		t.Fatalf("community destination: %v", err)
	}

	for _, c := range []struct {
		name        string
		account     string
		site        Site
		destination string
		want        string
	}{
		{"a trusted page is named by its site", "player", SiteCommunity, steam, "player - Steam Community"},
		{"an untrusted page is named by its host", "player", SiteCommunity, "https://csrep.gg/player/1", "player - csrep.gg"},
		// Nothing to attribute the window to, but it still must not claim Steam.
		{"no account still names the host", "", SiteStore, "https://csrep.gg/", "csrep.gg"},
		// An address with no host at all cannot be described, so the site it was
		// opened on is the only thing left to say.
		{"an unreadable address falls back to the site", "player", SiteStore, "", "player - Steam Store"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := windowTitle(c.account, c.site, c.destination); got != c.want {
				t.Errorf("windowTitle = %q, want %q", got, c.want)
			}
		})
	}
}
