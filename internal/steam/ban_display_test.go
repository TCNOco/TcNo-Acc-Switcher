package steam

import "testing"

// Hiding a ban is a display preference for one account. It must not depend on,
// or quietly change, whether the app shows bans at all.
func TestBanDisplayFor(t *testing.T) {
	banned := VacEntry{Vac: true}
	limited := VacEntry{Ltd: true}
	clean := VacEntry{}

	cases := []struct {
		name                        string
		entry                       VacEntry
		showVAC, showLimited, hide  bool
		wantVAC, wantLtd, wantOffer bool
	}{
		{
			// The show flags are the display settings, not a claim about this
			// account: they stay on, there is simply no ban to paint.
			name: "clean account offers nothing", entry: clean, showVAC: true, showLimited: true,
			wantVAC: true, wantLtd: true,
		},
		{
			name: "VAC ban shown", entry: banned, showVAC: true, showLimited: true,
			wantVAC: true, wantLtd: true, wantOffer: true,
		},
		{
			// Still offered, so the user can put it back.
			name: "VAC ban hidden for this account", entry: banned, showVAC: true, showLimited: true, hide: true,
			wantOffer: true,
		},
		{
			// Nothing to hide when the setting already hides it everywhere.
			name: "VAC ban with the global setting off", entry: banned, showVAC: false, showLimited: true,
			wantLtd: true,
		},
		{
			name: "limited account follows its own setting", entry: limited, showVAC: false, showLimited: true,
			wantLtd: true, wantOffer: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := banDisplayFor(tc.entry, tc.showVAC, tc.showLimited, tc.hide)
			if got.ShowVAC != tc.wantVAC || got.ShowLimited != tc.wantLtd {
				t.Errorf("show VAC/limited = %v/%v, want %v/%v", got.ShowVAC, got.ShowLimited, tc.wantVAC, tc.wantLtd)
			}
			if got.HasVisibleBan != tc.wantOffer {
				t.Errorf("HasVisibleBan = %v, want %v", got.HasVisibleBan, tc.wantOffer)
			}
			if got.Hidden != tc.hide {
				t.Errorf("Hidden = %v, want %v", got.Hidden, tc.hide)
			}
		})
	}
}
