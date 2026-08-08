// Package registry stores a non-secret display hint for Steam account rows.
// Callers must revalidate every protected operation against the encrypted vault.
package registry

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

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

const (
	Version       = 1
	maxIndexBytes = 128 << 10
	maxEntries    = 512
	fileName      = "registrations.json"
)

var (
	ErrInvalidIndex = errors.New("invalid Steam Guard registration index")
	ErrRootNotReady = errors.New("Steam Guard vault root is not ready")

	// writeMu serializes the load-mutate-save cycles below. Callers span lock
	// domains - the enrollment and background scan paths hold no service mutex at
	// all - so two concurrent writes would otherwise drop an entry or a state
	// transition, silently losing the lock icon until some later write rewrote it.
	writeMu sync.Mutex
)

type State string

const (
	StatePending State = "pending"
	StateActive  State = "active"
	// StateLoginOnly is an account whose vault record holds a Steam session but
	// no authenticator. It has no code to show and no confirmations to approve,
	// but it is in the vault and can be signed in again.
	StateLoginOnly State = "login_only"
)

type Entry struct {
	SteamID64 string `json:"steamId64"`
	State     State  `json:"state"`
}

type indexFile struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

func RootPath() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "SteamGuard"), nil
}

func IndexPath() (string, error) {
	root, err := RootPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, fileName), nil
}

func Load() ([]Entry, error) {
	path, err := IndexPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxIndexBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxIndexBytes {
		return nil, ErrInvalidIndex
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var stored indexFile
	if err := dec.Decode(&stored); err != nil {
		return nil, errors.Join(ErrInvalidIndex, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF || stored.Version != Version || len(stored.Entries) > maxEntries {
		return nil, ErrInvalidIndex
	}
	seen := make(map[string]struct{}, len(stored.Entries))
	for _, entry := range stored.Entries {
		if !validSteamID(entry.SteamID64) || entry.State == "" {
			return nil, ErrInvalidIndex
		}
		if _, exists := seen[entry.SteamID64]; exists {
			return nil, ErrInvalidIndex
		}
		seen[entry.SteamID64] = struct{}{}
	}
	sortEntries(stored.Entries)
	return stored.Entries, nil
}

func Status(steamID64 string) (hasSteamGuard, pending bool) {
	entries, err := Load()
	if err != nil {
		return false, false
	}
	for _, entry := range entries {
		if entry.SteamID64 == strings.TrimSpace(steamID64) {
			return entry.State == StateActive, entry.State == StatePending
		}
	}
	return false, false
}

// InVault reports whether the account has any vault record at all, whatever its
// state. An unrecognised state still counts: the record exists, this build just
// cannot say what it holds.
func (e Entry) InVault() bool { return e.State != "" }

func Upsert(steamID64 string, state State) error {
	steamID64 = strings.TrimSpace(steamID64)
	if !validSteamID(steamID64) || !validState(state) {
		return ErrInvalidIndex
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	entries, err := Load()
	if err != nil {
		return err
	}
	found := false
	for i := range entries {
		if entries[i].SteamID64 == steamID64 {
			entries[i].State = state
			found = true
			break
		}
	}
	if !found {
		if len(entries) == maxEntries {
			return ErrInvalidIndex
		}
		entries = append(entries, Entry{SteamID64: steamID64, State: state})
	}
	return save(entries)
}

func Remove(steamID64 string) error {
	steamID64 = strings.TrimSpace(steamID64)
	if !validSteamID(steamID64) {
		return ErrInvalidIndex
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	entries, err := Load()
	if err != nil {
		return err
	}
	next := entries[:0]
	for _, entry := range entries {
		if entry.SteamID64 != steamID64 {
			next = append(next, entry)
		}
	}
	return save(next)
}

func save(entries []Entry) error {
	sortEntries(entries)
	raw, err := json.MarshalIndent(indexFile{Version: Version, Entries: entries}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	root, err := RootPath()
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrRootNotReady
		}
		return err
	}
	if !info.IsDir() {
		return ErrRootNotReady
	}
	path := filepath.Join(root, fileName)
	return fsutil.WriteFileAtomic(path, raw, 0o600)
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].SteamID64 < entries[j].SteamID64 })
}

// validState gates what this build may WRITE. Reading is deliberately more
// tolerant: Load keeps an unrecognised state verbatim and save round-trips it,
// so an index written by a newer build degrades to "no icon" for the accounts it
// does not understand instead of failing the whole file - which would have
// blanked every account's Steam Guard state, not just the unfamiliar ones.
func validState(state State) bool {
	return state == StatePending || state == StateActive || state == StateLoginOnly
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
