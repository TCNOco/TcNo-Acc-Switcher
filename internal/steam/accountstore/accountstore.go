// Package accountstore is the switcher's own record of the Steam accounts it
// has seen. loginusers.vdf belongs to Steam, and Steam - or Advanced Clearing,
// or any cleaning tool - may truncate it at any moment. This store is additive:
// accounts found in that file are imported and updated, and an account only
// leaves the store when the user forgets it explicitly.
//
// It holds display metadata only, never credentials, tokens or secrets; those
// belong in the encrypted Steam Guard vault. Staying plaintext is what lets the
// account list render without a vault unlock, the same trade-off the Steam
// Guard registration index already makes.
//
// This package must not import internal/steam or internal/steamguard: both
// import it, and internal/steamguard already imports internal/steam.
package accountstore

import (
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
	fileName     = "accounts.json"
	maxFileBytes = 512 << 10
	maxEntries   = 512

	// platformKey duplicates internal/steam.PlatformKey, which this package
	// cannot import. order.json is resolved the same way.
	platformKey = "Steam"

	// lastSeenGranularity stops the loginusers.vdf sync rewriting the file on
	// every account-list rebuild. The list rebuilds on every window focus, and
	// LastSeenInVDF is diagnostic, so an hour of drift costs nothing.
	lastSeenGranularity = time.Hour
)

// Source is where an account was first seen. It is provenance, not current
// state: an account first imported from the vault keeps SourceSteamGuard even
// once Steam knows about it too.
type Source string

const (
	SourceVDF        Source = "vdf"
	SourceSteamGuard Source = "steamguard"
	SourceManual     Source = "manual"
)

var (
	ErrInvalidStore = errors.New("invalid Steam account store")
	ErrStoreFull    = errors.New("Steam account store is full")

	// writeMu serializes the load-mutate-save cycles. Writers span lock domains
	// - the account list holds the service read lock, Steam Guard enrollment
	// holds none - so two concurrent cycles would otherwise drop an account.
	writeMu sync.Mutex
)

// Record is one known Steam account. The vdf-shaped fields are stored verbatim
// so a switch can rebuild the account's loginusers.vdf row byte-for-byte.
type Record struct {
	SteamID64   string `json:"steamId64"`
	AccountName string `json:"accountName,omitempty"`
	PersonaName string `json:"personaName,omitempty"`
	// Timestamp is Steam's own last-login epoch, copied verbatim, so a row
	// Steam has forgotten still shows a last-used date.
	Timestamp       string `json:"timestamp,omitempty"`
	WantsOffline    string `json:"wantsOffline,omitempty"`
	SkipOfflineWarn string `json:"skipOfflineWarn,omitempty"`

	Source Source `json:"source,omitempty"`
	// FirstSeen and LastSeenInVDF are Unix seconds. LastSeenInVDF is zero for
	// an account Steam has never listed.
	FirstSeen     int64 `json:"firstSeen,omitempty"`
	LastSeenInVDF int64 `json:"lastSeenInVdf,omitempty"`
}

type storeFile struct {
	Version int      `json:"version"`
	Entries []Record `json:"entries"`
}

// Path is <DataRoot>/LoginCache/Steam/accounts.json.
func Path() (string, error) {
	dir, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// Load returns the stored accounts, or an empty slice when the file does not
// exist yet.
//
// A malformed file is an error and the file is left untouched. Every caller
// degrades to whatever loginusers.vdf holds and none of them writes afterwards,
// so a bad parse can never truncate the user's account history - it only costs
// them the accounts Steam has since forgotten, until the file is repaired.
func Load() ([]Record, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
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

	// Unknown fields are tolerated deliberately. registrations.json rejects
	// them because its worst case is a missing lock icon; the worst case here
	// is an empty account list, so a file a newer build added a field to must
	// still read. A version bump is the escape hatch for shape changes.
	var stored storeFile
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, errors.Join(ErrInvalidStore, err)
	}
	if stored.Version != Version {
		return nil, ErrInvalidStore
	}
	if len(stored.Entries) > maxEntries {
		return nil, ErrInvalidStore
	}

	seen := make(map[string]struct{}, len(stored.Entries))
	out := make([]Record, 0, len(stored.Entries))
	for _, rec := range stored.Entries {
		rec.SteamID64 = strings.TrimSpace(rec.SteamID64)
		if !validSteamID64(rec.SteamID64) {
			continue
		}
		if _, dup := seen[rec.SteamID64]; dup {
			continue
		}
		seen[rec.SteamID64] = struct{}{}
		out = append(out, rec)
	}
	sortRecords(out)
	return out, nil
}

// Get returns one record. The bool is false when the account is not stored.
func Get(steamID64 string) (Record, bool, error) {
	steamID64 = strings.TrimSpace(steamID64)
	stored, err := Load()
	if err != nil {
		return Record{}, false, err
	}
	for _, rec := range stored {
		if rec.SteamID64 == steamID64 {
			return rec, true, nil
		}
	}
	return Record{}, false, nil
}

// Save replaces the whole store.
func Save(records []Record) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	return save(records)
}

// Upsert merges one record into the store, reporting whether it moved.
func Upsert(rec Record) (changed bool, err error) {
	return UpsertMany([]Record{rec})
}

// UpsertMany merges records in a single load-save cycle and reports whether the
// file actually changed. The account list rebuilds on every window focus, so
// the unchanged case must not touch the disk.
func UpsertMany(records []Record) (bool, error) {
	if len(records) == 0 {
		return false, nil
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	stored, err := Load()
	if err != nil {
		return false, err
	}
	next, changed := mergeRecords(stored, records, time.Now().Unix())
	if !changed {
		return false, nil
	}
	if err := save(next); err != nil {
		return false, err
	}
	return true, nil
}

// Remove drops one account. Forgetting an account is the only way it leaves.
func Remove(steamID64 string) error {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return ErrInvalidStore
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	stored, err := Load()
	if err != nil {
		return err
	}
	next := make([]Record, 0, len(stored))
	found := false
	for _, rec := range stored {
		if rec.SteamID64 == steamID64 {
			found = true
			continue
		}
		next = append(next, rec)
	}
	if !found {
		return nil
	}
	return save(next)
}

// mergeRecords folds incoming onto stored and reports whether anything moved.
// It is pure so the sync path can be tested without touching a disk.
func mergeRecords(stored, incoming []Record, nowUnix int64) ([]Record, bool) {
	out := append([]Record(nil), stored...)
	index := make(map[string]int, len(out))
	for i, rec := range out {
		index[rec.SteamID64] = i
	}

	changed := false
	for _, rec := range incoming {
		rec.SteamID64 = strings.TrimSpace(rec.SteamID64)
		if !validSteamID64(rec.SteamID64) {
			continue
		}
		i, ok := index[rec.SteamID64]
		if !ok {
			out = append(out, newRecord(rec, nowUnix))
			index[rec.SteamID64] = len(out) - 1
			changed = true
			continue
		}
		if merged, diff := mergeInto(out[i], rec, nowUnix); diff {
			out[i] = merged
			changed = true
		}
	}
	return out, changed
}

func newRecord(rec Record, nowUnix int64) Record {
	rec.AccountName = strings.TrimSpace(rec.AccountName)
	rec.PersonaName = strings.TrimSpace(rec.PersonaName)
	rec.Timestamp = strings.TrimSpace(rec.Timestamp)
	rec.WantsOffline = strings.TrimSpace(rec.WantsOffline)
	rec.SkipOfflineWarn = strings.TrimSpace(rec.SkipOfflineWarn)
	if rec.FirstSeen == 0 {
		rec.FirstSeen = nowUnix
	}
	if rec.Source == SourceVDF && rec.LastSeenInVDF == 0 {
		rec.LastSeenInVDF = nowUnix
	}
	return rec
}

// mergeInto applies src's non-empty fields onto dst. An empty incoming field
// leaves the stored one alone, so a Steam Guard enrollment - which knows only
// the ID and login name - cannot erase the persona and last-login date an
// earlier loginusers.vdf sync recorded.
func mergeInto(dst, src Record, nowUnix int64) (Record, bool) {
	out := dst
	diff := false
	diff = overwriteIfSet(&out.AccountName, src.AccountName) || diff
	diff = overwriteIfSet(&out.PersonaName, src.PersonaName) || diff
	diff = overwriteIfSet(&out.Timestamp, src.Timestamp) || diff
	diff = overwriteIfSet(&out.WantsOffline, src.WantsOffline) || diff
	diff = overwriteIfSet(&out.SkipOfflineWarn, src.SkipOfflineWarn) || diff

	// Provenance and first-seen are write-once: they answer "where did this
	// account come from", which a later sighting cannot change.
	if out.Source == "" && src.Source != "" {
		out.Source = src.Source
		diff = true
	}
	if out.FirstSeen == 0 {
		out.FirstSeen = nowUnix
		diff = true
	}
	if src.Source == SourceVDF && nowUnix-out.LastSeenInVDF >= int64(lastSeenGranularity/time.Second) {
		out.LastSeenInVDF = nowUnix
		diff = true
	}
	return out, diff
}

func overwriteIfSet(dst *string, src string) bool {
	src = strings.TrimSpace(src)
	if src == "" || *dst == src {
		return false
	}
	*dst = src
	return true
}

// save assumes writeMu is held.
func save(records []Record) error {
	seen := make(map[string]struct{}, len(records))
	clean := make([]Record, 0, len(records))
	for _, rec := range records {
		rec.SteamID64 = strings.TrimSpace(rec.SteamID64)
		if !validSteamID64(rec.SteamID64) {
			continue
		}
		if _, dup := seen[rec.SteamID64]; dup {
			continue
		}
		seen[rec.SteamID64] = struct{}{}
		clean = append(clean, rec)
	}
	if len(clean) > maxEntries {
		return ErrStoreFull
	}
	sortRecords(clean)

	raw, err := json.MarshalIndent(storeFile{Version: Version, Entries: clean}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw, 0o600)
}

// sortRecords keeps the file, and the tail of the account list, deterministic.
// SteamID64 order is roughly account-creation order.
func sortRecords(records []Record) {
	sort.Slice(records, func(i, j int) bool { return records[i].SteamID64 < records[j].SteamID64 })
}

// validSteamID64 accepts the individual-account range only. Anything else could
// not be switched to, so storing it would just be a row that never works.
func validSteamID64(value string) bool {
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
