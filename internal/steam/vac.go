package steam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

// VacEntry matches SteamVACCache.json rows.
type VacEntry struct {
	SteamID string `json:"SteamID"`
	Vac     bool   `json:"Vac"`
	Ltd     bool   `json:"Ltd"`
}

func vacCachePath() (string, error) {
	r, err := paths.LoginCacheDir("Steam")
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "VACCache", "SteamVACCache.json"), nil
}

// LoadVacCache reads the cache; deletes the file if older than maxAgeDays (same semantics as C# DeletedOutdatedFile).
func LoadVacCache(maxAgeDays int) ([]VacEntry, error) {
	p, err := vacCachePath()
	if err != nil {
		return nil, err
	}
	if maxAgeDays > 0 {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if time.Since(st.ModTime()) > time.Duration(maxAgeDays)*24*time.Hour {
				_ = os.Remove(p)
				return nil, nil
			}
		}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []VacEntry
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// SaveVacCache writes SteamVACCache.json.
func SaveVacCache(rows []VacEntry) error {
	p, err := vacCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, data, 0o644)
}

func vacMap(rows []VacEntry) map[string]VacEntry {
	m := make(map[string]VacEntry, len(rows))
	for _, r := range rows {
		m[r.SteamID] = r
	}
	return m
}
