package steamguard

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steamguard/cs2ranks"
)

const collectorAccount = "76561198000000077"

func useCollectorRoot(t *testing.T) {
	t.Helper()
	paths.ResetForTest(tempDir(t))
}

func fullEntry() cs2ranks.Entry {
	return cs2ranks.Entry{PremierRating: 15234, PremierWins: 42, WingmanRank: 11, WingmanWins: 8, CompRank: 18}
}

func collect(t *testing.T) ([]byte, error) {
	t.Helper()
	return collectCS2Ranks(context.Background(), collectorAccount)
}

func TestCollectorServesAStoredRank(t *testing.T) {
	useCollectorRoot(t)
	if err := cs2ranks.Put(collectorAccount, fullEntry(), time.Now()); err != nil {
		t.Fatal(err)
	}
	body, err := collect(t)
	if err != nil {
		t.Fatalf("collectCS2Ranks: %v", err)
	}
	var payload map[string]int
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload: %v (%s)", err, body)
	}
	for key, want := range map[string]int{"premier": 15234, "premierWins": 42, "wingman": 11, "wingmanWins": 8, "compRank": 18} {
		if payload[key] != want {
			t.Fatalf("payload[%q] = %d, want %d (%s)", key, payload[key], want, body)
		}
	}
}

func TestCollectorDeclinesWithoutAnEntry(t *testing.T) {
	// The chain reads this as an ordinary variant failure and moves to Leetify.
	useCollectorRoot(t)
	if _, err := collect(t); !errors.Is(err, basic.ErrGameStatsCollectorNoData) {
		t.Fatalf("err = %v, want ErrGameStatsCollectorNoData", err)
	}
}

func TestCollectorDeclinesWhenItCannotSupplyWhatThePublicAPIsWould(t *testing.T) {
	// Claiming the variant stops the chain, so a partial answer is not "one
	// source missing a metric" - it is the metric missing from the tile. Better
	// to decline and let Leetify serve exactly what it serves today.
	cases := map[string]cs2ranks.Entry{
		"no premier":   {PremierRating: -1, CompRank: 18, WingmanRank: 11},
		"no comp rank": {PremierRating: 15234, CompRank: -1, WingmanRank: 11},
		"neither":      {PremierRating: -1, CompRank: -1, WingmanRank: 11},
	}
	for name, entry := range cases {
		useCollectorRoot(t)
		if err := cs2ranks.Put(collectorAccount, entry, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := collect(t); !errors.Is(err, basic.ErrGameStatsCollectorNoData) {
			t.Fatalf("%s: err = %v, want the collector to decline", name, err)
		}
	}
}

func TestCollectorDeclinesAStaleEntry(t *testing.T) {
	// An account dropped from the vault must eventually stop showing a rank
	// nobody is refreshing, and hand back to the public APIs.
	useCollectorRoot(t)
	if err := cs2ranks.Put(collectorAccount, fullEntry(), time.Now().Add(-cs2ranks.MaxAge-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := collect(t); !errors.Is(err, basic.ErrGameStatsCollectorNoData) {
		t.Fatalf("err = %v, want the collector to decline a stale entry", err)
	}
}

func TestCollectorOmitsAWingmanItDoesNotHave(t *testing.T) {
	// Wingman is additive: an account that has never played it must render no
	// Wingman tile rather than rank 0, but must still get the other metrics.
	useCollectorRoot(t)
	entry := fullEntry()
	entry.WingmanRank = -1
	entry.WingmanWins = -1
	if err := cs2ranks.Put(collectorAccount, entry, time.Now()); err != nil {
		t.Fatal(err)
	}
	body, err := collect(t)
	if err != nil {
		t.Fatalf("collectCS2Ranks: %v", err)
	}
	var payload map[string]int
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["wingman"]; ok {
		t.Fatalf("absent Wingman was serialised: %s", body)
	}
	if payload["premier"] != 15234 {
		t.Fatalf("other metrics lost: %s", body)
	}
}
