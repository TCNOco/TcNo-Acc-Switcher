package basic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

func gameStatsCacheRoot() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "StatsCache"), nil
}

func gameStatsCachePath(game string) (string, error) {
	root, err := gameStatsCacheRoot()
	if err != nil {
		return "", err
	}
	safe := paths.SanitizePathSegment(game)
	if safe == "" {
		safe = "game"
	}
	return filepath.Join(root, safe+".json"), nil
}

func (m *gameStatsManager) loadGameCacheLocked(game string) error {
	p, err := gameStatsCachePath(game)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			if m.cacheByGame[game] == nil {
				m.cacheByGame[game] = map[string]userGameStat{}
			}
			return nil
		}
		return fmt.Errorf("read %s: %w", p, err)
	}
	var cached map[string]userGameStat
	if err := json.Unmarshal(data, &cached); err != nil {
		// Corrupt cache should not block normal usage.
		m.cacheByGame[game] = map[string]userGameStat{}
		return nil
	}
	if cached == nil {
		cached = map[string]userGameStat{}
	}
	for accountID, row := range cached {
		if row.Vars == nil {
			row.Vars = map[string]string{}
		}
		if row.Collected == nil {
			row.Collected = map[string]string{}
		}
		row.HiddenMetrics = normalizeHiddenMetrics(row.HiddenMetrics, m.defs[game].Collect)
		cached[accountID] = row
	}
	m.cacheByGame[game] = cached
	gameStatsLog.Debug("loaded game stats cache", "game", game, "accounts", len(cached), "path", p)
	return nil
}

func (m *gameStatsManager) saveGameCacheLocked(game string) error {
	p, err := gameStatsCachePath(game)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
	}
	payload := m.cacheByGame[game]
	if payload == nil {
		payload = map[string]userGameStat{}
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	for acct, row := range payload {
		if !row.LastUpdated.IsZero() && row.LastUpdated.Before(cutoff) {
			delete(payload, acct)
		}
	}
	if len(payload) == 0 {
		delete(m.cacheByGame, game)
	} else {
		m.cacheByGame[game] = payload
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	gameStatsLog.Debug("saving game stats cache", "game", game, "accounts", len(payload), "path", p)
	return fsutil.WriteFileAtomic(p, b, 0o644)
}
