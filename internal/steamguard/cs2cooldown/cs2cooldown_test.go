package cs2cooldown

import (
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

const testID = "76561198000000001"

var testNow = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

func useStoreRoot(t *testing.T) {
	t.Helper()
	paths.ResetForTest(t.TempDir())
}

func TestLoadReturnsEmptyWhenTheFileIsAbsent(t *testing.T) {
	useStoreRoot(t)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load = %#v, want empty", got)
	}
}

func TestPutAndLoadRoundTrip(t *testing.T) {
	useStoreRoot(t)
	expiry := testNow.Add(72 * time.Hour)
	if err := Put(testID, expiry, false, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	entries, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	entry := entries[testID]
	if entry.CooldownExpiresAt != expiry.Unix() || entry.Permanent || entry.CheckedAt != testNow.Unix() {
		t.Fatalf("entry = %#v", entry)
	}
	if !entry.Active(testNow) {
		t.Fatal("Active = false, want true")
	}
}

func TestPutStoresAPermanentCooldownWithoutAnExpiry(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, time.Time{}, true, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	entries, _ := Load()
	entry := entries[testID]
	if !entry.Permanent || entry.CooldownExpiresAt != 0 {
		t.Fatalf("entry = %#v", entry)
	}
	// A permanent cooldown must stay active however far the clock runs.
	if !entry.Active(testNow.Add(100 * 365 * 24 * time.Hour)) {
		t.Fatal("Active = false, want true for a permanent cooldown")
	}
}

func TestClearKeepsTheEntryAndZeroesTheExpiry(t *testing.T) {
	useStoreRoot(t)
	// The entry has to survive so CheckedAt keeps driving the request floor.
	if err := Put(testID, testNow.Add(time.Hour), false, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	later := testNow.Add(2 * time.Hour)
	if err := Clear(testID, later); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	entries, _ := Load()
	entry, ok := entries[testID]
	if !ok {
		t.Fatal("entry was deleted, want it kept with a zeroed expiry")
	}
	if entry.CooldownExpiresAt != 0 || entry.Permanent || entry.CheckedAt != later.Unix() {
		t.Fatalf("entry = %#v", entry)
	}
	if entry.Active(later) {
		t.Fatal("Active = true after Clear")
	}
}

func TestAnExpiredEntryIsNotActive(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, testNow.Add(time.Hour), false, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	entries, _ := Load()
	if entries[testID].Active(testNow.Add(2 * time.Hour)) {
		t.Fatal("Active = true past the expiry")
	}
}

func TestPutRejectsAnInvalidSteamID(t *testing.T) {
	useStoreRoot(t)
	for _, id := range []string{"", "  ", "0", "1", "not-a-number", "076561198000000001"} {
		if err := Put(id, testNow, false, testNow); err == nil {
			t.Fatalf("Put(%q) succeeded, want error", id)
		}
	}
}

func TestPutNormalisesSurroundingWhitespace(t *testing.T) {
	useStoreRoot(t)
	// Input is trimmed the way registry.Upsert trims, but storage stays strict:
	// Load rejects a non-canonical id, so the two must agree.
	if err := Put("  "+testID+"  ", testNow.Add(time.Hour), false, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	entries, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := entries[testID]; !ok {
		t.Fatalf("entries = %#v, want the trimmed id", entries)
	}
}

func TestLoadRejectsAMalformedFile(t *testing.T) {
	cases := map[string]string{
		"unknown field":  `{"version":1,"entries":[],"extra":true}`,
		"wrong version":  `{"version":2,"entries":[]}`,
		"trailing value": `{"version":1,"entries":[]} {}`,
		"bad steam id":   `{"version":1,"entries":[{"steamId64":"nope","cooldownExpiresAt":0,"permanent":false,"checkedAt":0}]}`,
		"duplicate id": `{"version":1,"entries":[` +
			`{"steamId64":"76561198000000001","cooldownExpiresAt":0,"permanent":false,"checkedAt":0},` +
			`{"steamId64":"76561198000000001","cooldownExpiresAt":0,"permanent":false,"checkedAt":0}]}`,
		"negative expiry": `{"version":1,"entries":[{"steamId64":"76561198000000001","cooldownExpiresAt":-1,"permanent":false,"checkedAt":0}]}`,
		"not json":        `nonsense`,
	}
	for name, body := range cases {
		useStoreRoot(t)
		path, err := Path()
		if err != nil {
			t.Fatalf("Path: %v", err)
		}
		if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if _, err := Load(); err == nil {
			t.Fatalf("%s: Load succeeded, want error", name)
		}
	}
}

func TestConcurrentPutsKeepEveryEntry(t *testing.T) {
	useStoreRoot(t)
	const count = 24
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := strconv.FormatUint(76561198000000001+uint64(i), 10)
			if err := Put(id, testNow.Add(time.Hour), false, testNow); err != nil {
				t.Errorf("Put: %v", err)
			}
		}(i)
	}
	wg.Wait()
	entries, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != count {
		t.Fatalf("len(entries) = %d, want %d", len(entries), count)
	}
}

func TestSaveEvictsTheLeastRecentlyChecked(t *testing.T) {
	useStoreRoot(t)
	// Accounts forgotten from the switcher stop being swept; without eviction
	// their stale entries would hold the cap against accounts still in use.
	for i := 0; i <= maxEntries; i++ {
		id := strconv.FormatUint(76561198000000001+uint64(i), 10)
		if err := Put(id, testNow.Add(time.Hour), false, testNow.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	entries, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != maxEntries {
		t.Fatalf("len(entries) = %d, want %d", len(entries), maxEntries)
	}
	if _, ok := entries["76561198000000001"]; ok {
		t.Fatal("oldest entry survived eviction")
	}
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
