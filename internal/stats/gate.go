package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
)

const appSettingsFile = "TcNo-Acc-Switcher.settings.json"

var (
	collectionCached atomic.Bool
	collectionReady  atomic.Bool
)

// SetStatsCollectionEnabled updates the in-memory cached value so collectionEnabled
// does not need to read the settings file from disk on every call.
// The platform package calls this when the user toggles the setting.
func SetStatsCollectionEnabled(enabled bool) {
	collectionCached.Store(enabled)
	collectionReady.Store(true)
}

// collectionEnabled mirrors AppSettings.StatsEnabled (default true when unset).
func collectionEnabled() bool {
	if collectionReady.Load() {
		return collectionCached.Load()
	}
	v := readCollectionEnabled()
	collectionCached.Store(v)
	collectionReady.Store(true)
	return v
}

func readCollectionEnabled() bool {
	dir, err := resolveExeDir()
	if err != nil {
		return true
	}
	data, err := os.ReadFile(filepath.Join(dir, appSettingsFile))
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return true
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return true
	}
	v, ok := raw["statsEnabled"]
	if !ok {
		return true
	}
	var b bool
	if err := json.Unmarshal(v, &b); err != nil {
		return true
	}
	return b
}
