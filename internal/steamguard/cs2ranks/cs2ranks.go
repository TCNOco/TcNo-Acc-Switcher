// Package cs2ranks stores each account's CS2 matchmaking standings.
//
// Steam puts Premier and Wingman on the same GCPD page as the competitive
// cooldown, so the cooldown sweep gets them for free - no extra request, no
// extra rate-limit exposure. They are kept here rather than fetched on demand
// because the page needs an authenticated session, which only exists while the
// Steam Guard vault happens to be unlocked; the account list has to be able to
// read a rank long after that.
//
// This is a cache of a first-party answer, not a source of truth: an account
// with no entry falls through to the unauthenticated stats providers.
package cs2ranks

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

const (
	Version      = 1
	maxFileBytes = 256 << 10
	maxEntries   = 512
	fileName     = "CS2Ranks.json"
	platformKey  = "Steam"
)

var (
	ErrInvalidStore = errors.New("invalid CS2 rank store")

	// writeMu serializes load-mutate-save, the same way the cooldown store does:
	// the sweep writes one account at a time from a background goroutine while
	// the stats collector reads on the UI path.
	writeMu sync.Mutex
)

// Entry is one account's last known standings.
//
// Absent values are stored as -1 rather than 0, because 0 is a real Wingman rank
// (unranked) and a real win count.
type Entry struct {
	SteamID64     string `json:"steamId64"`
	PremierRating int    `json:"premierRating"`
	PremierWins   int    `json:"premierWins"`
	WingmanRank   int    `json:"wingmanRank"`
	WingmanWins   int    `json:"wingmanWins"`
	// CompRank is Steam's overall competitive skill group, or the highest per-map
	// one where the page carries no overall figure - which is what the
	// third-party providers reduce to.
	CompRank int `json:"compRank"`
	// PrimeState is "prime", "nonprime", or empty for not known. Kept beside the
	// ranks because the same sweep produces it and the same list reads it.
	PrimeState string `json:"primeState,omitempty"`
	CheckedAt  int64  `json:"checkedAt"`
}

// MaxAge caps how long a stored rank may stand in for a live one.
//
// Lives on the store rather than at either call site: the stats chain and the
// account tile both decide whether to show a stored rank, and if they disagreed
// an account could show a Premier rating in one place and fall through to
// Leetify in the other.
const MaxAge = 14 * 24 * time.Hour

// Fresh reports whether the entry is recent enough to prefer over a third-party
// provider. A rank only moves when the account plays, and playing means
// switching to it, which unlocks the vault and refreshes this - so the window
// can be generous. The cap exists so an account dropped from the vault stops
// showing a rank that has quietly gone stale.
func (e Entry) Fresh(now time.Time, maxAge time.Duration) bool {
	if e.CheckedAt <= 0 {
		return false
	}
	return now.Sub(time.Unix(e.CheckedAt, 0)) <= maxAge
}

type storeFile struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

func Path() (string, error) {
	dir, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Load returns every stored entry keyed by SteamID64. A missing file is the
// normal state before the first sweep and is not an error.
func Load() (map[string]Entry, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Entry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxFileBytes {
		return nil, ErrInvalidStore
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var stored storeFile
	if err := dec.Decode(&stored); err != nil {
		return nil, errors.Join(ErrInvalidStore, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF || stored.Version != Version || len(stored.Entries) > maxEntries {
		return nil, ErrInvalidStore
	}
	out := make(map[string]Entry, len(stored.Entries))
	for _, entry := range stored.Entries {
		if !validSteamID(entry.SteamID64) || entry.CheckedAt < 0 {
			return nil, ErrInvalidStore
		}
		if _, exists := out[entry.SteamID64]; exists {
			return nil, ErrInvalidStore
		}
		out[entry.SteamID64] = entry
	}
	return out, nil
}

// Lookup reads one account's entry.
func Lookup(steamID64 string) (Entry, bool) {
	entries, err := Load()
	if err != nil {
		return Entry{}, false
	}
	entry, ok := entries[strings.TrimSpace(steamID64)]
	return entry, ok
}

// Put records an account's standings. Callers pass -1 for a value the page did
// not carry, so a missing Wingman row cannot be mistaken for rank 0.
func Put(steamID64 string, entry Entry, checkedAt time.Time) error {
	steamID64 = strings.TrimSpace(steamID64)
	if !validSteamID(steamID64) {
		return ErrInvalidStore
	}
	entry.SteamID64 = steamID64
	entry.CheckedAt = checkedAt.Unix()

	writeMu.Lock()
	defer writeMu.Unlock()
	entries, err := Load()
	if err != nil {
		return err
	}
	entries[steamID64] = entry
	return save(entries)
}

func save(entries map[string]Entry) error {
	list := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry)
	}
	// Evict least-recently-checked first, so accounts no longer swept cannot
	// hold the cap against ones still in use.
	if len(list) > maxEntries {
		sort.Slice(list, func(i, j int) bool { return list[i].CheckedAt > list[j].CheckedAt })
		list = list[:maxEntries]
	}
	sort.Slice(list, func(i, j int) bool { return list[i].SteamID64 < list[j].SteamID64 })

	raw, err := json.MarshalIndent(storeFile{Version: Version, Entries: list}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o600)
}

func validSteamID(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return false
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return false
	}
	const min = uint64(76561197960265728)
	const max = min + uint64(^uint32(0))
	return id >= min && id <= max && value == strconv.FormatUint(id, 10)
}
