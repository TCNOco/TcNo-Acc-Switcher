package steam

import (
	"os"
	"path/filepath"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

func miniprofileCachePath(steamID64 string) (string, error) {
	r, err := paths.LoginCacheDir("Steam")
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "MiniProfileCache", steamID64+".html"), nil
}

func ReadCachedMiniprofileHTML(steamID64 string) string {
	p, err := miniprofileCachePath(steamID64)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) == 0 {
		return ""
	}
	return sanitizeMiniprofileHTML(string(data))
}

func deleteMiniprofileCache(steamID64 string) {
	p, err := miniprofileCachePath(steamID64)
	if err != nil {
		return
	}
	_ = os.Remove(p)
}

func ClearAllMiniprofileHTMLCache() error {
	r, err := paths.LoginCacheDir("Steam")
	if err != nil {
		return err
	}
	dir := filepath.Join(r, "MiniProfileCache")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

func deleteMiniprofileCacheIfOlder(steamID64 string, maxAgeDays int) bool {
	p, err := miniprofileCachePath(steamID64)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	if profileimage.FileOlderThanDays(p, maxAgeDays) {
		_ = os.Remove(p)
		return true
	}
	return false
}
