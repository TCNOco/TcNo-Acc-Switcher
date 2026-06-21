package basic

import (
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/winutil"
)

const gameStatsProcessCacheInterval = 5 * time.Minute

var (
	gameStatsProcessCacheMu sync.RWMutex
	gameStatsProcessCache   map[string]struct{}
	gameStatsProcessCacheAt time.Time
)

func normalizeGameProcessName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToLower(name)
	if !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}
	return name
}

func refreshRunningProcessCache(force bool) {
	gameStatsProcessCacheMu.Lock()
	defer gameStatsProcessCacheMu.Unlock()
	if !force && gameStatsProcessCache != nil && time.Since(gameStatsProcessCacheAt) < gameStatsProcessCacheInterval {
		return
	}
	set, err := winutil.SnapshotRunningExeBasenames()
	if err != nil {
		gameStatsLog.Debug("game stats process snapshot failed", "err", err)
		return
	}
	gameStatsProcessCache = set
	gameStatsProcessCacheAt = time.Now()
}

func isGameProcessRunning(processName string) bool {
	processName = normalizeGameProcessName(processName)
	if processName == "" {
		return false
	}
	refreshRunningProcessCache(false)
	gameStatsProcessCacheMu.RLock()
	defer gameStatsProcessCacheMu.RUnlock()
	if gameStatsProcessCache == nil {
		return winutil.IsExeRunning(processName)
	}
	_, ok := gameStatsProcessCache[processName]
	return ok
}
