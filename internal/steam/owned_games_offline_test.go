package steam

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/steam/ownedgames"
)

// offlineIconDoer answers the way appclient.Shared does once offline mode is
// on: the request never leaves the process. It stands in for offline being
// switched on between EnsureGameIcon's own check and the request it makes.
type offlineIconDoer struct{ calls atomic.Int32 }

func (d *offlineIconDoer) Do(req *http.Request) (*http.Response, error) {
	d.calls.Add(1)
	return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: appclient.ErrOfflineMode}
}

// useEmptyLibraryCache pins the memoised librarycache lookup at an empty
// directory, so a test never resolves art out of the machine's own Steam
// install and pass or fail does not depend on what is installed.
func useEmptyLibraryCache(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	gameIconLibraryMu.Lock()
	prevDir, prevTime := gameIconLibraryDir, gameIconLibraryTime
	gameIconLibraryDir, gameIconLibraryTime = empty, time.Now()
	gameIconLibraryMu.Unlock()
	t.Cleanup(func() {
		gameIconLibraryMu.Lock()
		gameIconLibraryDir, gameIconLibraryTime = prevDir, prevTime
		gameIconLibraryMu.Unlock()
	})
}

// forgetGameIconMisses keeps the process-wide negative cache from leaking one
// test's app ids into the next.
func forgetGameIconMisses(t *testing.T, ids ...string) {
	t.Helper()
	t.Cleanup(func() {
		gameIconMissMu.Lock()
		for _, id := range ids {
			delete(gameIconMisses, id)
		}
		gameIconMissMu.Unlock()
	})
}

func goOffline(t *testing.T) {
	t.Helper()
	appclient.SetOfflineMode(true)
	t.Cleanup(func() { appclient.SetOfflineMode(false) })
}

func TestEnsureGameIconSkipsTheCDNWhileOffline(t *testing.T) {
	_, doer := useTempIconCache(t)
	useEmptyLibraryCache(t)
	forgetGameIconMisses(t, "731")
	goOffline(t)

	iconURL, err := EnsureGameIcon(context.Background(), "731")
	if err != nil || iconURL != "" {
		t.Fatalf("EnsureGameIcon = %q, %v; want the caller's placeholder and no error", iconURL, err)
	}
	if n := doer.calls.Load(); n != 0 {
		t.Fatalf("offline icon lookup made %d HTTP requests", n)
	}
	if gameIconRecentlyMissing("731") {
		t.Fatal("offline mode was cached as 'this app has no artwork'")
	}
}

func TestEnsureGameIconDoesNotCacheAMissWhenOfflineSwitchesOnMidDownload(t *testing.T) {
	// A refusal from the shared client says nothing about whether the CDN has
	// art for this app. Filing it as a miss would leave the icon absent for the
	// whole miss TTL, hours after the user came back online.
	useTempIconCache(t)
	useEmptyLibraryCache(t)
	forgetGameIconMisses(t, "732")
	doer := &offlineIconDoer{}
	gameIconClient = doer

	iconURL, err := EnsureGameIcon(context.Background(), "732")
	if iconURL != "" {
		t.Fatalf("EnsureGameIcon = %q, want empty", iconURL)
	}
	if !errors.Is(err, appclient.ErrOfflineMode) {
		t.Fatalf("err = %v, want an offline-mode error", err)
	}
	if gameIconRecentlyMissing("732") {
		t.Fatal("a refused download was cached as 'this app has no artwork'")
	}
}

func TestWarmGameIconsMakesNoRequestsWhileOffline(t *testing.T) {
	// The warm pass is fire-and-forget off a UI call, so nothing downstream
	// would report that it had spent a screenful of requests while offline.
	_, doer := useTempIconCache(t)
	useEmptyLibraryCache(t)
	ids := []string{"733", "734", "735"}
	forgetGameIconMisses(t, ids...)
	goOffline(t)

	if got := WarmGameIcons(context.Background(), ids); len(got) != 0 {
		t.Fatalf("offline warm resolved %d urls", len(got))
	}
	if n := doer.calls.Load(); n != 0 {
		t.Fatalf("offline warm made %d HTTP requests", n)
	}
}

func TestGetOwnedGamesListReturnsStoredLibrariesWhileOffline(t *testing.T) {
	// Offline must never read as "owns nothing": the store is local, and an
	// empty games view would look exactly like every account losing its library.
	useOwnedGamesRoot(t, []InstalledGameInfo{{AppID: "570", Name: "Dota 2"}})
	if err := ownedgames.Put(ownedGamesAccountA, []uint32{730, 440}, time.Now()); err != nil {
		t.Fatal(err)
	}
	goOffline(t)

	list, err := NewSteamService().GetOwnedGamesList()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]OwnedGameDTO, len(list))
	for _, game := range list {
		byID[game.AppID] = game
	}
	if len(byID) != 3 {
		t.Fatalf("list = %#v, want the two stored games and the installed one", list)
	}
	if owners := byID["730"].Owners; len(owners) != 1 || owners[0] != ownedGamesAccountA {
		t.Fatalf("owners of 730 = %v, want the stored account", owners)
	}
	// The name map is cached on disk, so offline costs the list its refresh, not
	// its names.
	if got := byID["440"].Name; got != "Team Fortress 2" {
		t.Fatalf("name of 440 = %q, want the cached name", got)
	}
}
