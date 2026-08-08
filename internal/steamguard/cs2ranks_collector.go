package steamguard

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/cs2ranks"
)

// CS2RankCollectorName is the value a GameStats.json variant puts in "Fetch" to
// be served from the authenticated sweep instead of a third-party API.
const CS2RankCollectorName = "steam-cs2-gcpd"

// cs2RankPayload is the shape the collector hands the stats chain.
//
// It deliberately mimics a small JSON API response so the GCPD variant's Collect
// entries stay "Source": "json" and reuse the very same DisplayAs, colour bands
// and formatting as the Leetify variant - one rendering path, one place to
// change how a rank looks.
type cs2RankPayload struct {
	Premier     int `json:"premier,omitempty"`
	PremierWins int `json:"premierWins,omitempty"`
	CompRank    int `json:"compRank,omitempty"`
	Wingman     int `json:"wingman,omitempty"`
	WingmanWins int `json:"wingmanWins,omitempty"`
}

// CS2GameName is the game these ranks belong to, as GameStats.json names it.
const CS2GameName = "Counter-Strike 2"

// RegisterCS2RankCollector makes the sweep's stored ranks available to the game
// stats chain. Call once at startup.
func RegisterCS2RankCollector() {
	basic.RegisterGameStatsCollector(CS2RankCollectorName, collectCS2Ranks)
	// The same payload, for accounts that never set CS2 stats up. Registering
	// both is what makes "Show CS2 rank" work whether or not the user has
	// configured game stats, with one rendering path either way.
	basic.RegisterStandaloneStats(steam.PlatformKey, CS2GameName, standaloneCS2Ranks)
}

// standaloneCS2Ranks serves the stored ranks to an account with no configured
// CS2 stats row. Same payload, same gate; only the caller differs.
func standaloneCS2Ranks(accountID string) ([]byte, bool) {
	payload, err := collectCS2Ranks(context.Background(), accountID)
	if err != nil || len(payload) == 0 {
		return nil, false
	}
	return payload, true
}

// collectCS2Ranks serves one account's stored standings.
//
// It reads the store only: no vault access, no network, no credentials. That is
// what makes a locked vault a non-event here - locking stops the sweep from
// refreshing, but the last authenticated rank still renders instantly on the
// account list.
//
// An error means "nothing usable for this account", which the chain treats as an
// ordinary variant failure and answers by moving on to Leetify, then CSRep. That
// single return is the whole "signed in? authenticated : public API" decision.
func collectCS2Ranks(ctx context.Context, accountID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	settings, err := steam.LoadSettings()
	if err != nil || !settings.SteamShowCS2Rank {
		return nil, basic.ErrGameStatsCollectorNoData
	}
	entry, ok := cs2ranks.Lookup(accountID)
	if !ok {
		return nil, basic.ErrGameStatsCollectorNoData
	}
	if !entry.Fresh(time.Now(), cs2ranks.MaxAge) {
		return nil, fmt.Errorf("%w: stored CS2 rank is stale", basic.ErrGameStatsCollectorNoData)
	}

	// Claiming this variant means the chain stops here, so the third-party
	// sources are never consulted for this account. Anything this collector
	// cannot supply is therefore not "missing from one source", it is missing
	// from the tile - so decline unless we have at least what the public APIs
	// already show. Wingman is additive and never gates the decision.
	if entry.PremierRating <= 0 || entry.CompRank <= 0 {
		return nil, fmt.Errorf("%w: stored CS2 ranks are incomplete", basic.ErrGameStatsCollectorNoData)
	}

	// -1 is the store's "the page did not carry this" marker. Leaving those
	// fields at zero lets omitempty drop them, so a metric is simply absent
	// rather than rendered as rank 0.
	payload := cs2RankPayload{Premier: entry.PremierRating, CompRank: entry.CompRank}
	if entry.PremierWins > 0 {
		payload.PremierWins = entry.PremierWins
	}
	if entry.WingmanRank > 0 {
		payload.Wingman = entry.WingmanRank
	}
	if entry.WingmanWins > 0 {
		payload.WingmanWins = entry.WingmanWins
	}
	return json.Marshal(payload)
}
