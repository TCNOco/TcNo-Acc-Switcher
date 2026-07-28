package steam

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
)

func writeProfileAsset(t *testing.T, name string) string {
	t.Helper()
	www, err := paths.WwwrootDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(www, "img", "profiles", "steam")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Caches written before the frame and avatar were localized still carry Steam
// URLs the webview's CSP refuses, and a cleared profiles folder leaves cached
// HTML pointing at deleted files. Both must read as stale so a refetch heals
// them; a localized cache whose files exist stays served from disk.
func TestMiniprofileCachedAssetsLocal(t *testing.T) {
	paths.ResetForTest(t.TempDir())

	remote := `<div class="miniprofile_playersection">` +
		`<div class="playersection_avatar_frame"><img src="https://shared.akamai.steamstatic.com/community_assets/images/items/1/f.png"></div>` +
		`<div class="playersection_avatar border_color_offline"><img src="https://avatars.akamai.steamstatic.com/abc_medium.jpg"></div>` +
		`</div>`
	if miniprofileCachedAssetsLocal(remote) {
		t.Fatal("remote frame and avatar reported as local")
	}

	framePath := writeProfileAsset(t, "76561198000000000_frame.png")
	writeProfileAsset(t, "76561198000000000_static.jpg")
	local := `<div class="miniprofile_playersection">` +
		`<div class="playersection_avatar_frame"><img src="/img/profiles/steam/76561198000000000_frame.png"></div>` +
		`<div class="playersection_avatar"><img src="/img/profiles/steam/76561198000000000_static.jpg"></div>` +
		`</div>`
	if !miniprofileCachedAssetsLocal(local) {
		t.Fatal("localized cache with existing files reported as stale")
	}

	// A frame with no img (no equipped frame) must not read as stale.
	noFrame := `<div class="miniprofile_playersection">` +
		`<div class="playersection_avatar"><img src="/img/profiles/steam/76561198000000000_static.jpg"></div>` +
		`</div>`
	if !miniprofileCachedAssetsLocal(noFrame) {
		t.Fatal("frameless cache reported as stale")
	}

	// Deleting the profiles folder must invalidate the cache so the assets are
	// downloaded again.
	if err := os.Remove(framePath); err != nil {
		t.Fatal(err)
	}
	if miniprofileCachedAssetsLocal(local) {
		t.Fatal("cache with a deleted asset reported as usable")
	}
}
