package basic

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGameStatTTL_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	var def gameDefinition
	if err := json.Unmarshal([]byte(`{"TTL":"3h"}`), &def); err != nil {
		t.Fatal(err)
	}
	if def.TTL.duration() != 3*time.Hour {
		t.Fatalf("got %v", def.TTL.duration())
	}
	if err := json.Unmarshal([]byte(`{"TTL":1800}`), &def); err != nil {
		t.Fatal(err)
	}
	if def.TTL.duration() != 30*time.Minute {
		t.Fatalf("got %v", def.TTL.duration())
	}
}

func TestGameStatTTL_Default(t *testing.T) {
	t.Parallel()
	def := gameDefinition{}
	if def.TTL.duration() != defaultGameStatTTL {
		t.Fatalf("got %v", def.TTL.duration())
	}
}

func TestGameStatEffectiveTTL(t *testing.T) {
	t.Parallel()
	def := gameDefinition{TTL: gameStatTTL(3 * time.Hour)}
	if got := gameStatEffectiveTTL(def, "a", "a"); got != 3*time.Hour {
		t.Fatalf("no process: got %v", got)
	}
	gameStatsProcessCacheMu.Lock()
	gameStatsProcessCache = map[string]struct{}{"cs2.exe": {}}
	gameStatsProcessCacheAt = time.Now()
	gameStatsProcessCacheMu.Unlock()
	def.ProcessName = "cs2.exe"
	if got := gameStatEffectiveTTL(def, "a", "b"); got != 3*time.Hour {
		t.Fatalf("non-live account: got %v", got)
	}
	if got := gameStatEffectiveTTL(def, "a", "a"); got != defaultGameRunningStatTTL {
		t.Fatalf("live + running: got %v want %v", got, defaultGameRunningStatTTL)
	}
}

func TestGameStatRowExpired(t *testing.T) {
	t.Parallel()
	fresh := userGameStat{LastUpdated: time.Now()}
	if gameStatRowExpired(fresh, 3*time.Hour) {
		t.Fatal("fresh row should not be expired")
	}
	stale := userGameStat{LastUpdated: time.Now().Add(-4 * time.Hour)}
	if !gameStatRowExpired(stale, 3*time.Hour) {
		t.Fatal("stale row should be expired")
	}
	if !gameStatRowExpired(userGameStat{}, 3*time.Hour) {
		t.Fatal("zero LastUpdated should be expired")
	}
}
