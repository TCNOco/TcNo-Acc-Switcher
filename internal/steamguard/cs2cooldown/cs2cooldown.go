// Package cs2cooldown stores each account's CS2 competitive cooldown expiry.
//
// A cooldown is an absolute instant, not a live value: once read, it stays
// correct while it counts down, so it only has to be collected when the vault
// happens to be unlocked. That is what lets the display keep working
// indefinitely without a background refresh.
//
// The file lives under LoginCache rather than beside the vault because the value
// must outlive the account's removal from the vault - a user who signs an
// account out of Steam Guard does not get to drop their cooldown with it. For
// the same reason there is no exported Remove: Clear zeroes the expiry but only
// on the strength of Steam saying the cooldown is gone.
package cs2cooldown

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
	maxFileBytes = 128 << 10
	maxEntries   = 512
	fileName     = "CS2Cooldowns.json"
	platformKey  = "Steam"
)

var (
	ErrInvalidStore = errors.New("invalid CS2 cooldown store")

	// writeMu serializes load-mutate-save. The sweep writes one account at a
	// time from a background goroutine while the account list reads the file on
	// the UI path, so without this a concurrent write would drop an entry.
	writeMu sync.Mutex
)

// Entry is one account's last known cooldown state.
//
// CooldownExpiresAt is a Unix second. Zero means "no cooldown", which is a
// positive statement - it is only written when Steam said so. CheckedAt records
// when we last got an answer, and drives both the per-account request floor and
// eviction.
type Entry struct {
	SteamID64         string `json:"steamId64"`
	CooldownExpiresAt int64  `json:"cooldownExpiresAt"`
	Permanent         bool   `json:"permanent"`
	CheckedAt         int64  `json:"checkedAt"`
}

// Active reports whether the entry describes a cooldown that is still running.
func (e Entry) Active(now time.Time) bool {
	if e.Permanent {
		return true
	}
	return e.CooldownExpiresAt > 0 && e.CooldownExpiresAt > now.Unix()
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

// Load returns every stored entry keyed by SteamID64.
//
// A missing file is not an error - it is the normal state before the first
// sweep. A corrupt one is, and the caller is expected to carry on with an empty
// map rather than failing the account list over a cache.
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
		if !validSteamID(entry.SteamID64) || entry.CooldownExpiresAt < 0 || entry.CheckedAt < 0 {
			return nil, ErrInvalidStore
		}
		if _, exists := out[entry.SteamID64]; exists {
			return nil, ErrInvalidStore
		}
		out[entry.SteamID64] = entry
	}
	return out, nil
}

// Put records a cooldown for steamID64. checkedAt is stamped by the caller so a
// whole sweep shares one instant.
func Put(steamID64 string, expiresAt time.Time, permanent bool, checkedAt time.Time) error {
	expiry := int64(0)
	if !permanent && !expiresAt.IsZero() {
		expiry = expiresAt.Unix()
	}
	return upsert(steamID64, expiry, permanent, checkedAt)
}

// Clear records that the account has no cooldown.
//
// The entry survives with a zeroed expiry rather than being deleted, so
// CheckedAt still holds and the per-account request floor keeps working.
func Clear(steamID64 string, checkedAt time.Time) error {
	return upsert(steamID64, 0, false, checkedAt)
}

func upsert(steamID64 string, expiresAt int64, permanent bool, checkedAt time.Time) error {
	steamID64 = strings.TrimSpace(steamID64)
	if !validSteamID(steamID64) {
		return ErrInvalidStore
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	entries, err := Load()
	if err != nil {
		return err
	}
	entries[steamID64] = Entry{
		SteamID64:         steamID64,
		CooldownExpiresAt: expiresAt,
		Permanent:         permanent,
		CheckedAt:         checkedAt.Unix(),
	}
	return save(entries)
}

func save(entries map[string]Entry) error {
	list := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry)
	}
	// Evict least-recently-checked first. Accounts forgotten from the switcher
	// stop being swept, so without this their entries would hold the cap against
	// accounts that are still in use.
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
