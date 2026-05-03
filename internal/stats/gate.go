package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const appSettingsFile = "TcNo-Acc-Switcher.settings.json"

// collectionEnabled mirrors AppSettings.StatsEnabled (default true when unset).
func collectionEnabled() bool {
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
