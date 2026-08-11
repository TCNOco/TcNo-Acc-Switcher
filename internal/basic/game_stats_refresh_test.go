package basic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

func useGameStatsRoot(t *testing.T) {
	t.Helper()
	exeDir := t.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
}

func writeIdsFileForTest(t *testing.T, platformKey string, ids map[string]string) {
	t.Helper()
	p, err := idsPath(platformKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(idsFile{IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A body that fetched and parsed but carried no metrics is what a captive portal
// or a half-resumed network answers with, and every account hits it in the same
// moment. Persisting the empty result wiped every tile at once.
func TestRefreshSaveKeepsPreviousStatsWhenNothingWasCollected(t *testing.T) {
	useGameStatsRoot(t)

	updated := time.Now().Add(-4 * time.Hour)
	m := &gameStatsManager{
		defs:   map[string]gameDefinition{"G": {}},
		compat: map[string][]string{"Steam": {"G"}},
		cacheByGame: map[string]map[string]userGameStat{
			"G": {"acct": {Collected: map[string]string{"Rank": "5"}, LastUpdated: updated}},
		},
	}

	err := m.refreshSaveLocked("Steam", "G", "acct", 0, []byte("<html></html>"), nil)
	if err == nil {
		t.Fatal("an empty collection must still be reported as a failure")
	}

	row := m.cacheByGame["G"]["acct"]
	if got := row.Collected["Rank"]; got != "5" {
		t.Fatalf("Collected[Rank] = %q, want the previous value 5", got)
	}
	if !row.LastUpdated.Equal(updated) {
		t.Fatalf("LastUpdated = %v, want it left stale at %v so the next round retries", row.LastUpdated, updated)
	}
}

func TestCollectGameStatsJobsForPlatformForcesFreshRows(t *testing.T) {
	useGameStatsRoot(t)
	writeIdsFileForTest(t, "Steam", map[string]string{"acct": "account"})

	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	loaded, defs, compat, cache := gameStatsState.loaded, gameStatsState.defs, gameStatsState.compat, gameStatsState.cacheByGame
	t.Cleanup(func() {
		gameStatsState.loaded, gameStatsState.defs = loaded, defs
		gameStatsState.compat, gameStatsState.cacheByGame = compat, cache
	})

	gameStatsState.loaded = true
	gameStatsState.defs = map[string]gameDefinition{"G": {}}
	gameStatsState.compat = map[string][]string{"Steam": {"G"}}
	gameStatsState.cacheByGame = map[string]map[string]userGameStat{
		"G": {"acct": {Collected: map[string]string{"Rank": "5"}, LastUpdated: time.Now()}},
	}

	if jobs := collectGameStatsJobsForPlatform("Steam", "", false); len(jobs) != 0 {
		t.Fatalf("stale collection returned %d jobs for a fresh row, want 0", len(jobs))
	}
	jobs := collectGameStatsJobsForPlatform("Steam", "", true)
	if len(jobs) != 1 || jobs[0].game != "G" || jobs[0].accountID != "acct" {
		t.Fatalf("forced collection = %#v, want one job for G/acct", jobs)
	}
}
