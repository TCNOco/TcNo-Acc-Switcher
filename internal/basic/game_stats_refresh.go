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

func emitGameStatsUpdated(p GameStatsUpdatedPatch) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(GameStatsUpdatedEvent, p)
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
		if err := refreshGameStatsWorker(platformKey, game, accountID); err != nil {
			gameStatsLog.Debug("background game stats refresh failed", "platform", platformKey, "game", game, "accountID", accountID, "err", err)
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
		if gameStatRowExpired(row, gameStatEffectiveTTL(def, accountID, liveAccountID)) {
			jobs = append(jobs, gameStatsRefreshJob{platformKey: platformKey, game: game, accountID: accountID})
		}
	}
	return jobs
}

func collectStaleGameStatsJobsForPlatform(platformKey, liveAccountID string) []gameStatsRefreshJob {
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
			if !gameStatRowExpired(row, gameStatEffectiveTTL(def, accountID, liveAccountID)) {
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
	liveID := currentLiveAccountID(b, platformKey)
	go func() {
		defer crashlog.Capture()
		gameStatsState.mu.Lock()
		if err := gameStatsState.ensureLoadedLocked(); err != nil {
			gameStatsState.mu.Unlock()
			return
		}
		jobs := collectStaleGameStatsJobsForPlatform(platformKey, liveID)
		gameStatsState.mu.Unlock()
		for _, job := range jobs {
			queueGameStatsRefresh(job.platformKey, job.game, job.accountID)
		}
		if len(jobs) > 0 {
			gameStatsLog.Debug("queued stale game stats refresh", "platform", platformKey, "jobs", len(jobs))
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
