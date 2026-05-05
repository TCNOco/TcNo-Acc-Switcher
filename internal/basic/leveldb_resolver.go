package basic

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/comparer"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tidwall/gjson"
)

const levelDBIdleTimeout = 2 * time.Minute

type levelDBReference struct {
	Path     string
	Key      string
	JSONPath string
}

type levelDBHandle struct {
	db         *leveldb.DB
	lastAccess time.Time
}

type levelDBStore struct {
	mu      sync.Mutex
	handles map[string]*levelDBHandle
}

type indexedDBComparer struct {
	base comparer.Comparer
}

var sharedLevelDBStore = &levelDBStore{handles: map[string]*levelDBHandle{}}

func init() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			sharedLevelDBStore.closeIdle(levelDBIdleTimeout)
		}
	}()
}

func isLevelDBReference(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "leveldb:")
}

func parseLevelDBReference(raw string) (levelDBReference, error) {
	raw = strings.TrimSpace(raw)
	if !isLevelDBReference(raw) {
		return levelDBReference{}, fmt.Errorf("bad leveldb reference format")
	}
	rest := strings.TrimSpace(raw[len("leveldb:"):])
	if rest == "" {
		return levelDBReference{}, fmt.Errorf("bad leveldb reference format")
	}
	pathSep := strings.Index(rest, ":")
	if len(rest) >= 2 && rest[1] == ':' {
		next := strings.Index(rest[2:], ":")
		if next < 0 {
			return levelDBReference{}, fmt.Errorf("bad leveldb reference format")
		}
		pathSep = 2 + next
	}
	if pathSep <= 0 || pathSep >= len(rest)-1 {
		return levelDBReference{}, fmt.Errorf("bad leveldb reference format")
	}
	pathPart := strings.TrimSpace(rest[:pathSep])
	rem := strings.TrimSpace(rest[pathSep+1:])
	if pathPart == "" || rem == "" {
		return levelDBReference{}, fmt.Errorf("bad leveldb reference format")
	}
	keyPart := rem
	jsonPath := ""
	if keySep := strings.LastIndex(rem, ":"); keySep >= 0 {
		candidatePath := strings.TrimSpace(rem[keySep+1:])
		if looksLikeGJSONPath(candidatePath) {
			keyPart = strings.TrimSpace(rem[:keySep])
			jsonPath = candidatePath
		}
	}
	ref := levelDBReference{
		Path:     pathPart,
		Key:      keyPart,
		JSONPath: jsonPath,
	}
	if ref.Path == "" || ref.Key == "" {
		return levelDBReference{}, fmt.Errorf("bad leveldb reference format")
	}
	return ref, nil
}

func looksLikeGJSONPath(s string) bool {
	if strings.Contains(s, "/") || strings.Contains(s, `\`) {
		return false
	}
	if strings.Contains(s, ".") || strings.Contains(s, "[") {
		return true
	}
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '_' {
				return false
			}
			continue
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func resolveLevelDBReference(raw string, ctx platform.PathTokenContext) (string, error) {
	ref, err := parseLevelDBReference(raw)
	if err != nil {
		return "", err
	}
	dbPath := platform.ExpandPathTokens(platform.ExpandWindowsPath(ref.Path), ctx)
	slog.Debug("leveldb resolve reference", "dbPath", dbPath, "key", ref.Key, "jsonPath", ref.JSONPath != "")
	return sharedLevelDBStore.readValueFresh(dbPath, ref.Key, ref.JSONPath)
}

func (s *levelDBStore) readValueFresh(dbPath, key, jsonPath string) (string, error) {
	dbPath = filepath.Clean(strings.TrimSpace(dbPath))
	if dbPath == "" {
		return "", fmt.Errorf("empty leveldb path")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("empty leveldb key")
	}
	slog.Debug("leveldb open db handle", "dbPath", dbPath, "source", "fresh-read")
	db, err := openReadOnlyLevelDB(dbPath)
	if err != nil {
		return "", fmt.Errorf("open leveldb %s: %w", dbPath, err)
	}
	defer func() {
		slog.Debug("leveldb close db handle", "dbPath", dbPath, "reason", "fresh-read")
		_ = db.Close()
	}()
	return readLevelDBValue(dbPath, db, key, jsonPath)
}

func (s *levelDBStore) readValue(dbPath, key, jsonPath string) (string, error) {
	dbPath = filepath.Clean(strings.TrimSpace(dbPath))
	if dbPath == "" {
		return "", fmt.Errorf("empty leveldb path")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("empty leveldb key")
	}
	s.mu.Lock()
	h, ok := s.handles[dbPath]
	if !ok {
		slog.Debug("leveldb open db handle", "dbPath", dbPath, "source", "cache-miss")
		db, err := openReadOnlyLevelDB(dbPath)
		if err != nil {
			s.mu.Unlock()
			return "", fmt.Errorf("open leveldb %s: %w", dbPath, err)
		}
		h = &levelDBHandle{db: db, lastAccess: time.Now()}
		s.handles[dbPath] = h
		logAllLevelDBKeys(dbPath, db)
	} else {
		slog.Debug("leveldb reuse db handle", "dbPath", dbPath, "source", "cache-hit")
	}
	h.lastAccess = time.Now()
	db := h.db
	s.mu.Unlock()

	return readLevelDBValue(dbPath, db, key, jsonPath)
}

func readLevelDBValue(dbPath string, db *leveldb.DB, key, jsonPath string) (string, error) {
	slog.Debug("leveldb read key", "dbPath", dbPath, "key", key, "jsonPath", jsonPath)
	keyBytes := []byte(key)
	b, err := db.Get(keyBytes, nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			slog.Debug("leveldb key exact miss, scanning fallback", "dbPath", dbPath, "requestedKey", key)
			if matchedKey, ok := findCompatibleLevelDBKey(db, keyBytes); ok {
				slog.Debug("leveldb read key fallback matched", "dbPath", dbPath, "requestedKey", key, "matchedKey", previewLevelDBValue(string(matchedKey)))
				b, err = db.Get(matchedKey, nil)
			}
		}
		if err != nil {
			slog.Debug("leveldb read key failed", "dbPath", dbPath, "key", key, "err", err)
			return "", fmt.Errorf("read leveldb key %q: %w", key, err)
		}
	}
	if strings.TrimSpace(jsonPath) == "" {
		v := strings.TrimSpace(string(b))
		slog.Debug("leveldb read key success", "dbPath", dbPath, "key", key, "valuePreview", previewLevelDBValue(v))
		return v, nil
	}
	res := gjson.GetBytes(b, jsonPath)
	v := strings.TrimSpace(res.String())
	slog.Debug("leveldb read key success json-select", "dbPath", dbPath, "key", key, "jsonPath", jsonPath, "valuePreview", previewLevelDBValue(v))
	return v, nil
}

func findCompatibleLevelDBKey(db *leveldb.DB, requested []byte) ([]byte, bool) {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	reqNorm := normalizeLevelDBKey(requested)
	reqText := strings.ToLower(string(reqNorm))
	if strings.Contains(reqText, "multiaccountstore") {
		reqText = "multiaccountstore"
	}
	var best []byte
	for iter.Next() {
		k := append([]byte(nil), iter.Key()...)
		if bytes.Equal(k, requested) || bytes.Contains(k, requested) {
			return k, true
		}
		kNorm := normalizeLevelDBKey(k)
		kText := strings.ToLower(string(kNorm))
		if bytes.Equal(kNorm, reqNorm) || bytes.Contains(kNorm, reqNorm) {
			if len(best) == 0 || len(k) < len(best) {
				best = k
			}
			continue
		}
		if reqText != "" && strings.Contains(kText, reqText) {
			if len(best) == 0 || len(k) < len(best) {
				best = k
			}
		}
	}
	if len(best) > 0 {
		return best, true
	}
	return nil, false
}

func normalizeLevelDBKey(k []byte) []byte {
	out := make([]byte, 0, len(k))
	for _, b := range k {
		// Strip all ASCII control bytes so keys with separators like \x00\x01
		// can match plain-text references from descriptor config.
		if b < 0x20 || b == 0x7f {
			continue
		}
		out = append(out, b)
	}
	out = bytes.ReplaceAll(out, []byte{'\\'}, []byte{'/'})
	return out
}

func logAllLevelDBKeys(dbPath string, db *leveldb.DB) {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	count := 0
	for iter.Next() {
		count++
		k := append([]byte(nil), iter.Key()...)
		slog.Debug("leveldb key", "dbPath", dbPath, "index", count, "key", previewLevelDBValue(string(k)))
	}
	if err := iter.Error(); err != nil {
		slog.Debug("leveldb key dump error", "dbPath", dbPath, "err", err)
		return
	}
	slog.Debug("leveldb key dump complete", "dbPath", dbPath, "count", count)
}

func openReadOnlyLevelDB(dbPath string) (*leveldb.DB, error) {
	slog.Debug("leveldb open read-only standard comparer", "dbPath", dbPath)
	db, err := leveldb.OpenFile(dbPath, &opt.Options{ReadOnly: true})
	if err == nil {
		slog.Debug("leveldb open read-only standard comparer success", "dbPath", dbPath)
		return db, nil
	}
	slog.Debug("leveldb open read-only standard comparer failed", "dbPath", dbPath, "err", err)
	defaultErr := err

	slog.Debug("leveldb open read-only indexeddb comparer", "dbPath", dbPath, "comparer", "idb_cmp1")
	db, err = leveldb.OpenFile(dbPath, &opt.Options{
		Comparer: indexedDBComparer{base: comparer.DefaultComparer},
		ReadOnly: true,
	})
	if err == nil {
		slog.Debug("leveldb open read-only indexeddb comparer success", "dbPath", dbPath, "comparer", "idb_cmp1")
		return db, nil
	}
	slog.Debug("leveldb open read-only indexeddb comparer failed", "dbPath", dbPath, "comparer", "idb_cmp1", "err", err)
	return nil, fmt.Errorf("standard comparer error: %w; indexeddb comparer error: %v", defaultErr, err)
}

func (c indexedDBComparer) Compare(a, b []byte) int {
	return c.base.Compare(a, b)
}

func (c indexedDBComparer) Name() string {
	return "idb_cmp1"
}

func (c indexedDBComparer) Separator(dst, a, b []byte) []byte {
	return c.base.Separator(dst, a, b)
}

func (c indexedDBComparer) Successor(dst, b []byte) []byte {
	return c.base.Successor(dst, b)
}

func (s *levelDBStore) closeIdle(maxIdle time.Duration) {
	cutoff := time.Now().Add(-maxIdle)
	var closeList []*leveldb.DB

	s.mu.Lock()
	for p, h := range s.handles {
		if h == nil || h.db == nil {
			delete(s.handles, p)
			continue
		}
		if h.lastAccess.Before(cutoff) {
			slog.Debug("leveldb close idle db handle", "dbPath", p, "idleBefore", cutoff.Format(time.RFC3339))
			closeList = append(closeList, h.db)
			delete(s.handles, p)
		}
	}
	s.mu.Unlock()

	for _, db := range closeList {
		_ = db.Close()
	}
}

func (s *levelDBStore) closeAll() {
	var closeList []*leveldb.DB
	s.mu.Lock()
	for p, h := range s.handles {
		if h != nil && h.db != nil {
			slog.Debug("leveldb close db handle", "dbPath", p, "reason", "close-all")
			closeList = append(closeList, h.db)
		}
		delete(s.handles, p)
	}
	s.mu.Unlock()
	for _, db := range closeList {
		_ = db.Close()
	}
}

func closeSharedLevelDBHandles(reason string) {
	slog.Debug("leveldb close shared handles requested", "reason", reason)
	sharedLevelDBStore.closeAll()
}

func previewLevelDBValue(s string) string {
	s = strings.TrimSpace(s)
	const max = 140
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
