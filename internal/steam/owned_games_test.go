package steam

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steam/ownedgames"
)

const (
	ownedGamesAccountA = "76561198000000201"
	ownedGamesAccountB = "76561198000000202"
)

// useOwnedGamesRoot points the store and the app name map at a temp root, and
// cuts the two seams that would otherwise reach the machine's Steam install and
// the artwork CDN.
func useOwnedGamesRoot(t *testing.T, installed []InstalledGameInfo) {
	t.Helper()
	paths.ResetForTest(t.TempDir())

	steamAppNameMapMu.Lock()
	steamAppNameMapMem = nil
	steamAppNameMapMu.Unlock()
	cachePath, err := appIdsUserPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(cachePath,
		[]byte(`{"730":"Counter-Strike 2","440":"Team Fortress 2","570":"Dota 2"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	installedFn, warmFn, localFn := ownedGamesInstalledFn, ownedGamesWarmFn, ownedGamesLocalIconsFn
	appInfoFn := appInfoNamesFn
	t.Cleanup(func() {
		ownedGamesInstalledFn, ownedGamesWarmFn = installedFn, warmFn
		ownedGamesLocalIconsFn = localFn
		appInfoNamesFn = appInfoFn
		steamAppNameMapMu.Lock()
		steamAppNameMapMem = nil
		steamAppNameMapMu.Unlock()
	})
	// Keeps whatever this machine's Steam client has cached out of the assertions.
	appInfoNamesFn = func() map[string]string { return nil }
	ownedGamesInstalledFn = func(map[string]string, map[string]string) []InstalledGameInfo { return installed }
	ownedGamesWarmFn = func(context.Context, []string) map[string]string { return nil }
	// Stands in for a librarycache that has art for everything, so the join is
	// tested rather than whatever Steam happens to have cached on this machine.
	ownedGamesLocalIconsFn = func(appIDs []string) map[string]string {
		out := make(map[string]string, len(appIDs))
		for _, id := range appIDs {
			out[id] = GameIconURL(id)
		}
		return out
	}
}

func TestGetOwnedGamesListJoinsOwnersAndInstalledGames(t *testing.T) {
	useOwnedGamesRoot(t, []InstalledGameInfo{
		{AppID: "570", Name: "Dota 2"},
		// Installed, owned by a vault account: it must not be duplicated, and it
		// keeps its owners.
		{AppID: "730", Name: "Counter-Strike 2"},
		// Installed with no name anywhere - the fallback has to match what
		// BuildInstalledGamesList produces.
		{AppID: "9999999", Name: "App 9999999"},
	})
	now := time.Now()
	if err := ownedgames.Put(ownedGamesAccountA, []uint32{730, 440}, now); err != nil {
		t.Fatal(err)
	}
	if err := ownedgames.Put(ownedGamesAccountB, []uint32{730}, now); err != nil {
		t.Fatal(err)
	}

	list, err := NewSteamService().GetOwnedGamesList()
	if err != nil {
		t.Fatal(err)
	}

	byID := make(map[string]OwnedGameDTO, len(list))
	names := make([]string, 0, len(list))
	for _, game := range list {
		if _, dup := byID[game.AppID]; dup {
			t.Fatalf("app %s appears twice", game.AppID)
		}
		byID[game.AppID] = game
		names = append(names, game.Name)
	}
	if len(byID) != 4 {
		t.Fatalf("list = %#v, want 4 games", list)
	}

	shared := byID["730"]
	if len(shared.Owners) != 2 || shared.Owners[0] != ownedGamesAccountA || shared.Owners[1] != ownedGamesAccountB {
		t.Fatalf("owners of 730 = %v, want both accounts", shared.Owners)
	}
	if shared.Name != "Counter-Strike 2" || shared.IconURL != GameIconURL("730") {
		t.Fatalf("730 = %#v", shared)
	}
	if owners := byID["440"].Owners; len(owners) != 1 || owners[0] != ownedGamesAccountA {
		t.Fatalf("owners of 440 = %v, want only account A", owners)
	}
	// Installed but owned by nobody in the vault: empty means ownership unknown,
	// which is a different statement from "nobody owns it".
	if owners := byID["570"].Owners; owners == nil || len(owners) != 0 {
		t.Fatalf("owners of installed-only 570 = %v, want an empty slice", owners)
	}
	if got := byID["9999999"].Name; got != "App 9999999" {
		t.Fatalf("unnamed app = %q, want the App <id> fallback", got)
	}

	want := []string{"App 9999999", "Counter-Strike 2", "Dota 2", "Team Fortress 2"}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("order = %v, want %v", names, want)
		}
	}
}

func TestGameStorePageURL(t *testing.T) {
	if got := gameStorePageURL("1091500"); got != "https://store.steampowered.com/app/1091500/" {
		t.Fatalf("gameStorePageURL(1091500) = %q", got)
	}
	// The result is handed to cmd.exe, so anything that could carry a shell
	// metacharacter, a second URL or a path escape has to come back empty rather
	// than be trimmed into shape.
	for _, appID := range []string{
		"",
		"   ",
		"0",
		"0730",
		"73a0",
		"730 & calc.exe",
		"730/../../etc",
		"730%2F",
		"-730",
		"12345678901",
		"https://evil.example/",
	} {
		if got := gameStorePageURL(appID); got != "" {
			t.Fatalf("gameStorePageURL(%q) = %q, want it refused", appID, got)
		}
	}
}
