package basic

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"

	_ "modernc.org/sqlite"
)

const latestModifiedFilePrefix = "LATEST_MODIFIED_FILE:"
const sqlitePrefix = "SQLITE:"

func resolveLatestModifiedFileValue(v, folder string, ctx platform.PathTokenContext) (string, bool, error) {
	trimmed := strings.TrimSpace(v)
	if !strings.HasPrefix(strings.ToUpper(trimmed), latestModifiedFilePrefix) {
		return "", false, nil
	}
	pattern := strings.TrimSpace(trimmed[len(latestModifiedFilePrefix):])
	if pattern == "" {
		return "", true, fmt.Errorf("empty latest modified file pattern")
	}
	pattern = expandPlatformPath(pattern, folder, ctx)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", true, fmt.Errorf("glob latest modified file %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return "", true, nil
	}
	latestPath := ""
	var latestModTime int64
	for _, p := range matches {
		st, statErr := os.Stat(p)
		if statErr != nil || st.IsDir() {
			continue
		}
		mt := st.ModTime().UnixNano()
		if latestPath == "" || mt > latestModTime {
			latestPath = p
			latestModTime = mt
		}
	}
	return strings.TrimSpace(latestPath), true, nil
}

func resolveSQLiteValue(v, folder string, ctx platform.PathTokenContext) (string, bool, error) {
	trimmed := strings.TrimSpace(v)
	if !strings.HasPrefix(strings.ToUpper(trimmed), sqlitePrefix) {
		return "", false, nil
	}
	rest := strings.TrimSpace(trimmed[len(sqlitePrefix):])
	pipeIdx := strings.Index(rest, "|")
	if pipeIdx <= 0 {
		return "", true, fmt.Errorf("bad SQLITE format")
	}
	dbPath := strings.TrimSpace(rest[:pipeIdx])
	query := strings.TrimSpace(rest[pipeIdx+1:])
	if dbPath == "" || query == "" {
		return "", true, fmt.Errorf("bad SQLITE format")
	}
	expandedDBPath := expandPlatformPath(dbPath, folder, ctx)
	dsn := "file:" + strings.ReplaceAll(expandedDBPath, `\`, `/`) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return "", true, fmt.Errorf("open SQLITE db %q: %w", expandedDBPath, err)
	}
	defer func() { _ = db.Close() }()
	var value any
	row := db.QueryRow(query)
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return "", true, nil
		}
		return "", true, fmt.Errorf("query SQLITE db %q: %w", expandedDBPath, err)
	}
	switch x := value.(type) {
	case nil:
		return "", true, nil
	case string:
		return strings.TrimSpace(x), true, nil
	case []byte:
		return strings.TrimSpace(string(x)), true, nil
	default:
		return strings.TrimSpace(fmt.Sprint(x)), true, nil
	}
}
