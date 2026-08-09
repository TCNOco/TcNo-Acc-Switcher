package ownedgames

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

const testID = "76561198000000001"

var testNow = time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

func useStoreRoot(t *testing.T) {
	t.Helper()
	paths.ResetForTest(t.TempDir())
}

func TestPutSortsAndDeduplicates(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, []uint32{80, 10, 70, 10, 80}, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := Lookup(testID)
	if !ok {
		t.Fatal("Lookup reported no entry")
	}
	want := []uint32{10, 70, 80}
	if len(got.AppIDs) != len(want) {
		t.Fatalf("appIds = %v, want %v", got.AppIDs, want)
	}
	for i := range want {
		if got.AppIDs[i] != want[i] {
			t.Fatalf("appIds = %v, want %v", got.AppIDs, want)
		}
	}
	if got.CheckedAt != testNow.Unix() {
		t.Fatalf("checkedAt = %d, want %d", got.CheckedAt, testNow.Unix())
	}
}

// An empty library is how Steam answers a caller it will not authorise, so
// storing one would leave the account permanently blank in the games view.
func TestPutRejectsAnEmptyLibrary(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, nil, testNow); err == nil {
		t.Fatal("Put accepted an empty library")
	}
	if _, ok := Lookup(testID); ok {
		t.Fatal("an entry was written for an empty library")
	}
}

// A failed write must not erase what a previous sweep learned.
func TestPutRejectionLeavesAnExistingEntryIntact(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, []uint32{10, 70}, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := Put(testID, nil, testNow.Add(time.Hour)); err == nil {
		t.Fatal("Put accepted an empty library")
	}
	got, ok := Lookup(testID)
	if !ok || len(got.AppIDs) != 2 {
		t.Fatalf("entry = %#v, want the original two app ids", got)
	}
}

func TestRemoveIsSilentWhenTheAccountIsAbsent(t *testing.T) {
	useStoreRoot(t)
	if err := Remove(testID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestRemoveDropsTheAccount(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, []uint32{10}, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := Remove(testID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := Lookup(testID); ok {
		t.Fatal("entry survived Remove")
	}
}

func TestLoadRejectsADuplicateAccount(t *testing.T) {
	useStoreRoot(t)
	writeStore(t, storeFile{Version: Version, Entries: []Entry{
		{SteamID64: testID, AppIDs: []uint32{10}, CheckedAt: testNow.Unix()},
		{SteamID64: testID, AppIDs: []uint32{70}, CheckedAt: testNow.Unix()},
	}})
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted a duplicate account")
	}
}

func TestLoadRejectsAWrongVersion(t *testing.T) {
	useStoreRoot(t)
	writeStore(t, storeFile{Version: Version + 1, Entries: []Entry{
		{SteamID64: testID, AppIDs: []uint32{10}, CheckedAt: testNow.Unix()},
	}})
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted a future version")
	}
}

// Accounts still being swept must not lose their place to ones that stopped.
func TestSaveEvictsTheLeastRecentlyChecked(t *testing.T) {
	useStoreRoot(t)
	entries := make(map[string]Entry, maxEntries+1)
	base, err := strconv.ParseUint(testID, 10, 64)
	if err != nil {
		t.Fatalf("parse base id: %v", err)
	}
	for i := 0; i <= maxEntries; i++ {
		id := strconv.FormatUint(base+uint64(i), 10)
		entries[id] = Entry{SteamID64: id, AppIDs: []uint32{10}, CheckedAt: testNow.Unix() - int64(i)}
	}
	if err := save(entries); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != maxEntries {
		t.Fatalf("stored %d entries, want %d", len(loaded), maxEntries)
	}
	oldest := strconv.FormatUint(base+uint64(maxEntries), 10)
	if _, ok := loaded[oldest]; ok {
		t.Fatal("the least recently checked account survived eviction")
	}
}

func writeStore(t *testing.T, file storeFile) {
	t.Helper()
	raw, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
