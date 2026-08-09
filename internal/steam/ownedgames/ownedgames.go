// Package ownedgames stores the set of Steam apps each account owns.
//
// Owned apps can only be read with an authenticated session, which exists only
// while the Steam Guard vault happens to be unlocked. The games view has to
// render long after that, so the answer is cached here in plaintext - the same
// trade-off the Steam Guard registration index and the CS2 rank store already
// make. It holds app ids and nothing else: no names, no tokens, no session
// material.
//
// Names are deliberately not stored. internal/steam already keeps an app id ->
// name map in AppIdsUser.json, refreshed on its own schedule; duplicating names
// per account would multiply the same strings across every record and let the
// two copies disagree.
package ownedgames

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
	Version = 1
	// A large library runs to a few thousand app ids, and the file holds one
	// list per account, so this cap is far above the CS2 stores' 256 KiB.
	maxFileBytes = 16 << 20
	maxEntries   = 256
	// maxAppIDs bounds one account's list. Valve's own store caps a library well
	// below this; a longer list means a malformed or hostile file, not a
	// collector.
	maxAppIDs   = 32768
	fileName    = "OwnedGames.json"
	platformKey = "Steam"
)

var (
	ErrInvalidStore = errors.New("invalid owned games store")

	// writeMu serializes load-mutate-save: the sweep writes one account at a
	// time from a background goroutine while the games list reads on the UI path.
	writeMu sync.Mutex
)

// Entry is one account's last known library.
type Entry struct {
	SteamID64 string   `json:"steamId64"`
	AppIDs    []uint32 `json:"appIds"`
	CheckedAt int64    `json:"checkedAt"`
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
		if !validSteamID(entry.SteamID64) || entry.CheckedAt < 0 || len(entry.AppIDs) > maxAppIDs {
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

// Put records an account's library.
//
// An empty list is rejected rather than stored. Steam answers an unauthorised
// caller with an empty result rather than an error, so "owns nothing" and "was
// not allowed to look" are indistinguishable here - and caching the former
// would leave the account permanently blank in the games view. Use Remove to
// drop an account deliberately.
func Put(steamID64 string, appIDs []uint32, checkedAt time.Time) error {
	steamID64 = strings.TrimSpace(steamID64)
	if !validSteamID(steamID64) || len(appIDs) == 0 || len(appIDs) > maxAppIDs {
		return ErrInvalidStore
	}
	entry := Entry{SteamID64: steamID64, AppIDs: dedupeSorted(appIDs), CheckedAt: checkedAt.Unix()}

	writeMu.Lock()
	defer writeMu.Unlock()
	entries, err := Load()
	if err != nil {
		return err
	}
	entries[steamID64] = entry
	return save(entries)
}

// Remove drops an account, for when it leaves the vault. Absent is success:
// the caller wants the account gone, not a report on how it was already gone.
func Remove(steamID64 string) error {
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
	if _, ok := entries[steamID64]; !ok {
		return nil
	}
	delete(entries, steamID64)
	return save(entries)
}

func dedupeSorted(appIDs []uint32) []uint32 {
	out := append([]uint32(nil), appIDs...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	end := 0
	for i, id := range out {
		if i > 0 && id == out[end-1] {
			continue
		}
		out[end] = id
		end++
	}
	return out[:end]
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

	raw, err := json.Marshal(storeFile{Version: Version, Entries: list})
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
