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
	if isManual {
		if _, ok := avatars.CachedFilePath(steamID64); ok {
			return avatars.OlderThanDays(steamID64, maxAgeDays)
		}
		return true
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
