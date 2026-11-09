package steam

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func markCacheFileStale(path string) error {
	stale := time.Unix(0, 0)
	if err := os.Chtimes(path, stale, stale); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func markCacheFilesStale(dir, extension string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(entry.Name()), extension) {
			if err := markCacheFileStale(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func markVACProfileCachesStale() error {
	path, err := vacCachePath()
	if err != nil {
		return err
	}
	return markCacheFilesStale(filepath.Dir(path), ".xml")
}

func markAllMiniprofileCachesStale() error {
	path, err := miniprofileCachePath("unused")
	if err != nil {
		return err
	}
	return markCacheFilesStale(filepath.Dir(path), ".html")
}
