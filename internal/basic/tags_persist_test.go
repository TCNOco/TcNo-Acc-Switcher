package basic

import (
	"testing"
	"time"
)

// This package's tests never call paths.InitDataRoot, so idsPath cannot resolve
// and every writeIdsFile here fails. That makes it a faithful stand-in for the
// real failure this guards against: ids.json momentarily unwritable because a
// file scanner is holding it during the atomic rename.
func TestPersistPrunedTagsSurvivesUnwritableIdsFile(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	f := idsFile{
		IDs:      map[string]string{"u1": "One"},
		LastUsed: map[string]string{},
		Tags: map[string]tagFileEntry{
			"expired": {Name: "Expired", Color: "#111111", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)},
			"keep":    {Name: "Keep", Color: "#222222"},
		},
		AccountTags:        map[string][]string{"u1": {"expired", "keep"}},
		AccountTagExpiries: map[string]map[string]string{},
	}
	if !pruneExpiredTagsInFile(&f, now) {
		t.Fatal("fixture should have had something to prune")
	}

	// The read paths call this and then carry on, so a write error must be
	// swallowed: returning it blanks the account list and the tag map.
	persistPrunedTags("BasicTestPlatformWithNoDataRoot", f)

	// The in-memory prune is what the caller returns, so it has to survive the
	// failed write intact.
	if _, ok := f.Tags["expired"]; ok {
		t.Fatal("expired tag definition should have been pruned in memory")
	}
	if got := f.AccountTags["u1"]; len(got) != 1 || got[0] != "keep" {
		t.Fatalf("u1 tags = %#v, want only keep", got)
	}
}
