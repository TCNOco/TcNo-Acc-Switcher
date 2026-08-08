package cs2ranks

import (
	"os"
	"path/filepath"
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

func sampleEntry() Entry {
	return Entry{PremierRating: 15234, PremierWins: 42, WingmanRank: 11, WingmanWins: 8}
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

func TestPutAndLookupRoundTrip(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, sampleEntry(), testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := Lookup(testID)
	if !ok {
		t.Fatal("Lookup reported no entry")
	}
	want := sampleEntry()
	want.SteamID64 = testID
	want.CheckedAt = testNow.Unix()
	if got != want {
		t.Fatalf("entry = %#v, want %#v", got, want)
	}
}

func TestAbsentValuesRoundTripAsNegativeOne(t *testing.T) {
	// 0 is a real Wingman rank and a real win count, so "not on the page" has to
	// be a distinct value or an account that has never played Wingman would look
	// unranked.
	useStoreRoot(t)
	entry := Entry{PremierRating: 15234, PremierWins: 42, WingmanRank: -1, WingmanWins: -1}
	if err := Put(testID, entry, testNow); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, _ := Lookup(testID)
	if got.WingmanRank != -1 || got.WingmanWins != -1 {
		t.Fatalf("entry = %#v, want the absent markers kept", got)
	}
}

func TestFreshTracksTheMaxAge(t *testing.T) {
	useStoreRoot(t)
	if err := Put(testID, sampleEntry(), testNow); err != nil {
		t.Fatal(err)
	}
	got, _ := Lookup(testID)
	if !got.Fresh(testNow.Add(13*24*time.Hour), 14*24*time.Hour) {
		t.Fatal("Fresh = false inside the window")
	}
	if got.Fresh(testNow.Add(15*24*time.Hour), 14*24*time.Hour) {
		t.Fatal("Fresh = true past the window")
	}
}

func TestAnEntryWithoutACheckTimeIsNeverFresh(t *testing.T) {
	if (Entry{}).Fresh(testNow, 14*24*time.Hour) {
		t.Fatal("a zero entry reported fresh")
	}
}

func TestPutRejectsAnInvalidSteamID(t *testing.T) {
	useStoreRoot(t)
	for _, id := range []string{"", "  ", "0", "1", "not-a-number", "076561198000000001"} {
		if err := Put(id, sampleEntry(), testNow); err == nil {
			t.Fatalf("Put(%q) succeeded, want error", id)
		}
	}
}

func TestLoadRejectsAMalformedFile(t *testing.T) {
	cases := map[string]string{
		"unknown field":  `{"version":1,"entries":[],"extra":true}`,
		"wrong version":  `{"version":2,"entries":[]}`,
		"trailing value": `{"version":1,"entries":[]} {}`,
		"bad steam id":   `{"version":1,"entries":[{"steamId64":"nope","premierRating":0,"premierWins":0,"wingmanRank":0,"wingmanWins":0,"checkedAt":0}]}`,
		"not json":       `nonsense`,
	}
	for name, body := range cases {
		useStoreRoot(t)
		path, err := Path()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); err == nil {
			t.Fatalf("%s: Load succeeded, want error", name)
		}
	}
}

func TestLookupReportsMissingOnACorruptStore(t *testing.T) {
	// The account list must degrade to the third-party providers rather than
	// failing because a cache file went bad.
	useStoreRoot(t)
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("nonsense"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := Lookup(testID); ok {
		t.Fatal("Lookup reported an entry from a corrupt store")
	}
}
