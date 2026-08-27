package steam

import (
	"testing"

	"TcNo-Acc-Switcher/internal/profileimage"
)

// fakeLookup answers from fixed sets, so the branch table can be driven without
// touching the filesystem.
type fakeLookup struct {
	cached map[string]bool // account has a cached image
	stale  map[string]bool // that image is older than the age limit
	manual map[string]bool
}

func (f fakeLookup) FindCached(id string) (string, bool) {
	if f.cached[id] {
		return "/img/profiles/steam/" + id + ".jpg", true
	}
	return "", false
}

func (f fakeLookup) CachedFilePath(id string) (string, bool) {
	if f.cached[id] {
		return `C:\cache\` + id + ".jpg", true
	}
	return "", false
}

func (f fakeLookup) HasManualProfileMarker(id string) bool { return f.manual[id] }

// OlderThanDays reproduces the rule both real implementations share: nothing
// cached reads as stale, and a non-positive limit forces stale.
func (f fakeLookup) OlderThanDays(id string, days int) bool {
	if days <= 0 || !f.cached[id] {
		return true
	}
	return f.stale[id]
}

// steamAvatarPendingOriginal keeps the pre-refactor shape as an oracle: an
// explicit "is it cached" check followed by an age check, which the rewrite
// collapsed into a single OlderThanDays call per branch.
func steamAvatarPendingOriginal(avatars profileimage.Lookup, steamID64, miniProfileHTML string, useMiniProfile bool, maxAgeDays int, isManual bool) bool {
	// Not part of the oracle: nothing ever downloads a manual avatar, so the
	// age check the original applied here could only ever be answered "pending"
	// forever. TestManualAvatarIsNeverPending pins the replacement rule.
	if isManual {
		return false
	}
	if useMiniProfile {
		staticID := steamStaticAvatarID(steamID64)
		_, hasStatic := avatars.CachedFilePath(staticID)
		if !hasStatic || avatars.OlderThanDays(staticID, maxAgeDays) {
			return true
		}
		if ExtractMiniprofileAvatarMediaURL(miniProfileHTML) == "" {
			return false
		}
		if _, ok := avatars.CachedFilePath(steamID64); !ok {
			return true
		}
		return avatars.OlderThanDays(steamID64, maxAgeDays)
	}
	if _, ok := avatars.CachedFilePath(steamID64); ok {
		return avatars.OlderThanDays(steamID64, maxAgeDays)
	}
	return true
}

// TestSteamAvatarPendingMatchesOriginalLogic walks the whole branch table. The
// rewrite removed the separate existence checks, so every combination of
// cached/stale/manual/mini-profile has to produce the same verdict as before.
func TestSteamAvatarPendingMatchesOriginalLogic(t *testing.T) {
	const id = "76561198000000001"
	staticID := steamStaticAvatarID(id)

	htmls := map[string]string{
		"noMedia":   "",
		"withMedia": `<div><img class="avatar" src="https://cdn.example/avatar.webm"></div>`,
	}
	states := []bool{false, true}
	checked := 0

	for _, primaryCached := range states {
		for _, primaryStale := range states {
			for _, staticCached := range states {
				for _, staticStale := range states {
					for _, useMini := range states {
						for _, isManual := range states {
							for _, days := range []int{-1, 0, 7} {
								for htmlName, html := range htmls {
									lookup := fakeLookup{
										cached: map[string]bool{id: primaryCached, staticID: staticCached},
										stale:  map[string]bool{id: primaryStale, staticID: staticStale},
									}
									want := steamAvatarPendingOriginal(lookup, id, html, useMini, days, isManual)
									got := steamAvatarPending(lookup, id, html, useMini, days, isManual)
									if got != want {
										t.Fatalf("primary(cached=%v,stale=%v) static(cached=%v,stale=%v) mini=%v manual=%v days=%d html=%s: got %v, want %v",
											primaryCached, primaryStale, staticCached, staticStale, useMini, isManual, days, htmlName, got, want)
									}
									checked++
								}
							}
						}
					}
				}
			}
		}
	}
	if want := 2 * 2 * 2 * 2 * 2 * 2 * 3 * 2; checked != want {
		t.Fatalf("covered %d combinations, expected the full table of %d", checked, want)
	}
}

// TestManualAvatarIsNeverPending pins the rule that broke the tiles: nothing ever
// re-downloads a manual avatar to reset its age, so list builds reported an aged
// one pending while the refresh reported it not pending, and the tile fell back to
// the placeholder.
func TestManualAvatarIsNeverPending(t *testing.T) {
	const id = "76561198000000001"
	staticID := steamStaticAvatarID(id)

	for _, cached := range []bool{false, true} {
		for _, stale := range []bool{false, true} {
			for _, useMini := range []bool{false, true} {
				for _, days := range []int{-1, 0, 7} {
					lookup := fakeLookup{
						cached: map[string]bool{id: cached, staticID: cached},
						stale:  map[string]bool{id: stale, staticID: stale},
						manual: map[string]bool{id: true},
					}
					if steamAvatarPending(lookup, id, "", useMini, days, true) {
						t.Fatalf("manual avatar reported pending (cached=%v stale=%v mini=%v days=%d)",
							cached, stale, useMini, days)
					}
				}
			}
		}
	}
}

// TestManualAvatarDisplayIgnoresLeftoverStatic covers the second half of the
// same flicker: ChangeAccountImage only deletes the main stem, so the Steam
// avatar's <id>_static survives a drop. Offering it as the manual avatar's
// static form would both paint the replaced image and disagree with the
// refresh, which resolves the manual URL alone.
func TestManualAvatarDisplayIgnoresLeftoverStatic(t *testing.T) {
	const manual = "/img/profiles/steam/76561198000000001.png"
	const video = "/img/profiles/steam/76561198000000001.webm"

	if img, static := resolveManualAvatarDisplay(manual); img != manual || static != manual {
		t.Fatalf("manual image: got (%q, %q), want (%q, %q)", img, static, manual, manual)
	}
	// A video has no static form, so there is nothing to hand an <img> offline.
	if img, static := resolveManualAvatarDisplay(video); img != video || static != "" {
		t.Fatalf("manual video: got (%q, %q), want (%q, \"\")", img, static, video)
	}
}
