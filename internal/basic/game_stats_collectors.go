package basic

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// GameStatsCollector produces a stats body for one account without going over
// the network on the chain's behalf.
//
// It exists because a stat source can need per-account authentication, which the
// URL-and-static-cookie shape of a GameStats.json variant cannot express: the
// definition file is user-editable and ends up in debug dumps, so a bearer token
// must never pass through it. A collector instead reads whatever the owning
// package already gathered under its own credentials and hands back the same
// JSON a remote source would have returned, so everything downstream - parsing,
// DisplayAs, attribution, the fallback chain - is unchanged.
//
// Returning an error is the supported way to say "not available for this
// account". The chain treats it as an ordinary variant failure and moves on to
// the next source, which is what makes "authenticated if we have it, otherwise
// the public API" fall out with no extra orchestration.
type GameStatsCollector func(ctx context.Context, accountID string) ([]byte, error)

// ErrGameStatsCollectorNoData is the canonical "nothing for this account" error.
var ErrGameStatsCollectorNoData = errors.New("no collected stats for this account")

var (
	gameStatsCollectorsMu sync.RWMutex
	gameStatsCollectors   = map[string]GameStatsCollector{}
)

// RegisterGameStatsCollector wires a named collector that GameStats.json
// variants can select with "Fetch".
//
// Registration comes from outside this package - internal/basic imports neither
// internal/steam nor internal/steamguard - the same indirection
// SetLiveAccountIDResolver uses. Passing nil removes the collector.
func RegisterGameStatsCollector(name string, fn GameStatsCollector) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	gameStatsCollectorsMu.Lock()
	defer gameStatsCollectorsMu.Unlock()
	if fn == nil {
		delete(gameStatsCollectors, name)
		return
	}
	gameStatsCollectors[name] = fn
}

func gameStatsCollectorFor(name string) GameStatsCollector {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	gameStatsCollectorsMu.RLock()
	defer gameStatsCollectorsMu.RUnlock()
	return gameStatsCollectors[name]
}
