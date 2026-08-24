package basic

import (
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/security"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// GameStatsUpdatedEvent is emitted when cached game stats finish a background refresh.
const GameStatsUpdatedEvent = "basic-game-stats-updated"

// GameStatsUpdatedPatch identifies one account whose inline stats markup may have changed.
type GameStatsUpdatedPatch struct {
	PlatformKey string `json:"platformKey"`
	UniqueID    string `json:"uniqueId"`
}

type gameStatsRefreshJob struct {
	platformKey string
	game        string
	accountID   string
}

var (
	gameStatsRefreshPending sync.Map // string -> struct{}

	// gameStatsRefreshSlots caps how many stats downloads are in flight.
	//
	// Every cached row expires while the machine sleeps, so the first tick after
	// a resume queues every account-and-game pair at once, each walking a
	// fallback chain of its own. Unbounded that is dozens of simultaneous
	// requests to the same few hosts from a network that has only just come
	// back - the shape most likely to be answered with the failures this
	// refresh exists to avoid.
	gameStatsRefreshSlots = make(chan struct{}, 4)
)

func (b *BasicService) setGameStatsActivePlatform(platformKey string) {
	if b == nil {
		return
	}
	platformKey = strings.TrimSpace(platformKey)
	b.gameStatsActiveMu.Lock()
	b.gameStatsActivePlatform = platformKey
	b.gameStatsActiveMu.Unlock()
}

func (b *BasicService) getGameStatsActivePlatform() string {
	if b == nil {
		return ""
	}
	b.gameStatsActiveMu.RLock()
	active := strings.TrimSpace(b.gameStatsActivePlatform)
	b.gameStatsActiveMu.RUnlock()
	return active
}

func gameStatsRefreshKey(platformKey, game, accountID string) string {
	return strings.TrimSpace(platformKey) + "\x00" + strings.TrimSpace(game) + "\x00" + strings.TrimSpace(accountID)
}

// EmitGameStatsUpdated tells an open account list that one account's inline
// stats markup may have changed. Exported for producers outside this package -
// a standalone stats provider updates its source without going through the
// refresh queue, so nothing else would announce it.
func EmitGameStatsUpdated(platformKey, uniqueID string) {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	if platformKey == "" || uniqueID == "" {
		return
	}
	emitGameStatsUpdated(GameStatsUpdatedPatch{PlatformKey: platformKey, UniqueID: uniqueID})
}

func emitGameStatsUpdated(p GameStatsUpdatedPatch) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(GameStatsUpdatedEvent, p)
}

// QueueGameStatsRefresh downloads one account's stats for one game in the
// background, ignoring the cache lifetime.
//
// Exported for a producer that has just changed what a variant would answer
// with - the authenticated CS2 sweep is the one - since announcing the change
// alone only re-reads the row the chain already wrote.
func QueueGameStatsRefresh(platformKey, game, accountID string) {
	queueGameStatsRefresh(platformKey, game, accountID)
}

func queueGameStatsRefresh(platformKey, game, accountID string) {
	platformKey = strings.TrimSpace(platformKey)
	game = strings.TrimSpace(game)
	accountID = strings.TrimSpace(accountID)
	if platformKey == "" || game == "" || accountID == "" {
		return
	}
	if appclient.IsOfflineMode() {
		return
	}
	key := gameStatsRefreshKey(platformKey, game, accountID)
	if _, loaded := gameStatsRefreshPending.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	go func() {
		defer crashlog.Capture()
		defer gameStatsRefreshPending.Delete(key)
		gameStatsRefreshSlots <- struct{}{}
		defer func() { <-gameStatsRefreshSlots }()
		if err := refreshGameStatsWorker(platformKey, game, accountID); err != nil {
			gameStatsLog().Debug("background game stats refresh failed", "platform", platformKey, "game", game, "accountID", accountID, "err", err)
			return
		}
		emitGameStatsUpdated(GameStatsUpdatedPatch{PlatformKey: platformKey, UniqueID: accountID})
	}()
}

func collectStaleGameStatsJobs(platformKey, accountID, liveAccountID string) []gameStatsRefreshJob {
	platformKey = strings.TrimSpace(platformKey)
	accountID = strings.TrimSpace(accountID)
	if platformKey == "" || accountID == "" {
		return nil
	}
	var jobs []gameStatsRefreshJob
	for _, game := range gameStatsState.compat[platformKey] {
		def, ok := gameStatsState.defs[game]
		if !ok {
			continue
		}
		row, ok := gameStatsState.cacheByGame[game][accountID]
		if !ok {
			continue
		}
		if gameStatRowExpired(row, gameStatEffectiveTTL(def.variantAt(row.FallbackIndex), accountID, liveAccountID)) {
			jobs = append(jobs, gameStatsRefreshJob{platformKey: platformKey, game: game, accountID: accountID})
		}
	}
	return jobs
}

// collectGameStatsJobsForPlatform lists every enabled account-and-game pair on
// platformKey that should be downloaded. With force set the TTL is ignored, so
// the caller gets the whole platform rather than only its stale rows.
func collectGameStatsJobsForPlatform(platformKey, liveAccountID string, force bool) []gameStatsRefreshJob {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil
	}
	idf, err := readIdsFile(platformKey)
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var jobs []gameStatsRefreshJob
	for _, game := range gameStatsState.compat[platformKey] {
		def, ok := gameStatsState.defs[game]
		if !ok {
			continue
		}
		rows := gameStatsState.cacheByGame[game]
		if rows == nil {
			continue
		}
		for accountID, row := range rows {
			accountID = strings.TrimSpace(accountID)
			if accountID == "" {
				continue
			}
			if _, ok := idf.IDs[accountID]; !ok {
				continue
			}
			if !force && !gameStatRowExpired(row, gameStatEffectiveTTL(def.variantAt(row.FallbackIndex), accountID, liveAccountID)) {
				continue
			}
			key := gameStatsRefreshKey(platformKey, game, accountID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			jobs = append(jobs, gameStatsRefreshJob{platformKey: platformKey, game: game, accountID: accountID})
		}
	}
	return jobs
}

// StartGameStatsRefresh queues background downloads for enabled stats older than each game's TTL.
func (b *BasicService) StartGameStatsRefresh(platformKey string) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" || appclient.IsOfflineMode() || security.AppLocked() {
		return
	}
	b.setGameStatsActivePlatform(platformKey)
	queuePlatformGameStatsRefresh(platformKey, currentLiveAccountID(b, platformKey), false)
}

// ForceGameStatsRefresh downloads every enabled game's stats on platformKey now,
// ignoring each row's TTL.
//
// Unlocking the Steam Guard vault is what this exists for: the authenticated
// sweep behind it can answer for accounts it had nothing to say about a minute
// ago, and a newly added account has no reason to wait out a cache lifetime it
// was never part of. Exported as a package function because the callers - the
// Steam Guard service - hold no BasicService.
func ForceGameStatsRefresh(platformKey string) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" || appclient.IsOfflineMode() || security.AppLocked() {
		return
	}
	queuePlatformGameStatsRefresh(platformKey, "", true)
}

// RefreshAllGameStats is ForceGameStatsRefresh for the account page, which asks
// for it when the user presses F5.
//
// It marks the platform active on the way through, the way StartGameStatsRefresh
// does, so the process monitor keeps working on it afterwards - a refresh the
// user asked for is also the clearest statement of which platform they are
// looking at.
func (b *BasicService) RefreshAllGameStats(platformKey string) error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil
	}
	b.setGameStatsActivePlatform(platformKey)
	ForceGameStatsRefresh(platformKey)
	return nil
}

func queuePlatformGameStatsRefresh(platformKey, liveAccountID string, force bool) {
	go func() {
		defer crashlog.Capture()
		gameStatsState.mu.Lock()
		if err := gameStatsState.ensureLoadedLocked(); err != nil {
			gameStatsState.mu.Unlock()
			return
		}
		jobs := collectGameStatsJobsForPlatform(platformKey, liveAccountID, force)
		gameStatsState.mu.Unlock()
		for _, job := range jobs {
			queueGameStatsRefresh(job.platformKey, job.game, job.accountID)
		}
		if len(jobs) > 0 {
			gameStatsLog().Debug("queued game stats refresh", "platform", platformKey, "jobs", len(jobs), "forced", force)
		}
	}()
}

// StartGameStatsProcessMonitor periodically snapshots running processes (every 5m) and refreshes stale stats.
func (b *BasicService) StartGameStatsProcessMonitor() {
	go b.runGameStatsProcessMonitor()
}

func (b *BasicService) runGameStatsProcessMonitor() {
	tick := func() {
		if appclient.IsOfflineMode() || security.AppLocked() {
			return
		}
		activePlatform := b.getGameStatsActivePlatform()
		if activePlatform == "" {
			return
		}
		refreshRunningProcessCache(true)
		b.StartGameStatsRefresh(activePlatform)
	}
	tick()
	ticker := time.NewTicker(gameStatsProcessCacheInterval)
	defer ticker.Stop()
	for range ticker.C {
		tick()
	}
}
