package stats

import (
	"strings"
)

// SyncPlatformCounts updates stored per-platform counters. Pass accounts < 0 to leave Accounts unchanged.
func SyncPlatformCounts(platformName string, accounts, gameShortcuts, gameShortcutsHotbar int) error {
	if !collectionEnabled() {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	p := ensurePlatformLocked(platformName)
	row := state.SwitcherStats[p]
	if accounts >= 0 {
		row.Accounts = accounts
	}
	row.GameShortcuts = gameShortcuts
	row.GameShortcutsHotbar = gameShortcutsHotbar
	state.SwitcherStats[p] = row
	return saveLocked()
}

// SyncPlatformTagCounts updates stored per-platform tag & tagged-account counters.
func SyncPlatformTagCounts(platformName string, tags int, taggedAccounts int) error {
	if !collectionEnabled() {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	p := ensurePlatformLocked(platformName)
	row := state.SwitcherStats[p]
	row.Tags = tags
	row.TaggedAccounts = taggedAccounts
	state.SwitcherStats[p] = row
	return saveLocked()
}

// IncrementGamesLaunched records a game/app launch initiated from the shortcut bar (or equivalent).
func IncrementGamesLaunched(platformName string) error {
	if !collectionEnabled() {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	p := ensurePlatformLocked(platformName)
	row := state.SwitcherStats[p]
	row.GamesLaunched++
	state.SwitcherStats[p] = row
	return saveLocked()
}

func normPageKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "/" {
		return "/"
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	if len(s) > 160 {
		return s[:160]
	}
	return s
}

// RecordPageVisit increments visit count for a SPA path (e.g. "/", "/settings", "/platform/Steam").
func RecordPageVisit(pageKey string) error {
	if !collectionEnabled() {
		return nil
	}
	pageKey = normPageKey(pageKey)
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	ps := state.PageStats[pageKey]
	ps.Visits++
	state.PageStats[pageKey] = ps
	return saveLocked()
}

// AddPageTime adds seconds spent on a SPA path to rolling totals.
func AddPageTime(pageKey string, seconds int) error {
	if !collectionEnabled() || seconds <= 0 {
		return nil
	}
	pageKey = normPageKey(pageKey)
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	ps := state.PageStats[pageKey]
	ps.TotalTime += seconds
	state.PageStats[pageKey] = ps
	return saveLocked()
}
