package platforms

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// firstCacheValue shares glob discovery, recency ordering and open-cache reads
// across providers. The parser returns an empty value to try the next file.
func firstCacheValue(pattern string, fallbackPatterns []string, parse func([]byte) string) (string, error) {
	files, err := cacheDataFiles(pattern, fallbackPatterns...)
	log := slog.Default().With("component", "profile-image-provider")
	log.Debug("scanning cache files", "pattern", pattern, "fallbackPatterns", fallbackPatterns, "files", len(files), "err", err)
	if err != nil {
		return "", err
	}
	sortByModDesc(files)
	for _, path := range files {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			continue
		}
		data, err := readCacheFile(path)
		if err != nil {
			log.Debug("cache file read failed", "file", path, "err", err)
			continue
		}
		if value := strings.TrimSpace(parse(data)); value != "" {
			log.Debug("cache value found", "file", path)
			return value, nil
		}
	}
	return "", nil
}

// cacheDataFiles collects only the descriptor's explicit patterns. Callers sort
// the combined results by modification time before parsing cached responses.
func cacheDataFiles(pattern string, fallbackPatterns ...string) ([]string, error) {
	patterns := append([]string{pattern}, fallbackPatterns...)
	seen := make(map[string]bool)
	var files []string
	for _, candidate := range patterns {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		matches, err := filepath.Glob(candidate)
		if err != nil {
			return nil, err
		}
		for _, file := range matches {
			if !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	return files, nil
}

func sortByModDesc(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		ist, ierr := os.Stat(paths[i])
		jst, jerr := os.Stat(paths[j])
		if ierr != nil || jerr != nil {
			return paths[i] > paths[j]
		}
		return ist.ModTime().After(jst.ModTime())
	})
}
