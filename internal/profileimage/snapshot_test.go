package profileimage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
)

// The data root resolves once per process, so it is set for the whole package.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "profileimage-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "temp data root:", err)
		os.Exit(1)
	}
	paths.InitDataRoot(root)
	code := m.Run()
	_ = os.RemoveAll(root)
	os.Exit(code)
}

const benchPlatform = "Steam"

// seedProfileCache writes one avatar for 80% of accounts, leaving the rest
// uncached so their probes run the whole extension list and miss — the mix a
// real install has. Every seventh account also gets a manual-avatar lock.
func seedProfileCache(t testing.TB, accounts int) []string {
	t.Helper()
	dir, err := ProfileDir(benchPlatform)
	if err != nil {
		t.Fatalf("profile dir: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("reset profile dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}

	ids := make([]string, accounts)
	for i := range ids {
		id := fmt.Sprintf("7656119%010d", i)
		ids[i] = id
		if i%5 == 4 {
			continue
		}
		ext := "jpg"
		if i%3 == 0 {
			ext = "png" // third in probe order, so three stats to reach it
		}
		if err := os.WriteFile(filepath.Join(dir, id+"."+ext), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed avatar: %v", err)
		}
		if i%7 == 0 {
			if err := os.WriteFile(filepath.Join(dir, id+ManualProfileMarkerSuffix), nil, 0o644); err != nil {
				t.Fatalf("seed marker: %v", err)
			}
		}
	}
	return ids
}

// TestSnapshotMatchesPerAccountLookups is the correctness bar for replacing the
// per-account probes: the snapshot must answer identically for every account,
// including those with no cached image and those holding a manual lock.
func TestSnapshotMatchesPerAccountLookups(t *testing.T) {
	ids := seedProfileCache(t, 40)
	snapshot, err := NewSnapshot(benchPlatform)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	for _, id := range ids {
		wantURL, wantOK := FindCached(benchPlatform, id)
		gotURL, gotOK := snapshot.FindCached(id)
		if gotURL != wantURL || gotOK != wantOK {
			t.Fatalf("%s: FindCached = (%q,%v), want (%q,%v)", id, gotURL, gotOK, wantURL, wantOK)
		}

		wantPath, wantOK := CachedFilePath(benchPlatform, id)
		gotPath, gotOK := snapshot.CachedFilePath(id)
		if gotPath != wantPath || gotOK != wantOK {
			t.Fatalf("%s: CachedFilePath = (%q,%v), want (%q,%v)", id, gotPath, gotOK, wantPath, wantOK)
		}

		if got, want := snapshot.HasManualProfileMarker(id), HasManualProfileMarker(benchPlatform, id); got != want {
			t.Fatalf("%s: HasManualProfileMarker = %v, want %v", id, got, want)
		}

		// The account list treats an uncached account as stale, which is what
		// FileOlderThanDays returns for a path that does not exist.
		for _, days := range []int{-1, 0, 1, 3650} {
			want := FileOlderThanDays(wantPath, days)
			if !wantOK {
				want = true
			}
			if got := snapshot.OlderThanDays(id, days); got != want {
				t.Fatalf("%s: OlderThanDays(%d) = %v, want %v", id, days, got, want)
			}
		}
	}
}

// DirectLookup backs the paths that download avatars while they run, so its
// "nothing cached" rule has to match the snapshot's or the two would classify
// the same account differently.
func TestDirectLookupTreatsMissingEntriesAsStale(t *testing.T) {
	direct := DirectLookup("PlatformThatHasNoCacheDirectory")
	const absent = "76561198000000002"

	for _, days := range []int{-1, 0, 30} {
		if !direct.OlderThanDays(absent, days) {
			t.Fatalf("days=%d: an account with nothing cached must read as stale", days)
		}
	}
	if _, ok := direct.FindCached(absent); ok {
		t.Fatal("reported a cached image for a platform with no cache directory")
	}
	if direct.HasManualProfileMarker(absent) {
		t.Fatal("reported a manual marker for a platform with no cache directory")
	}
}

func TestSnapshotOnMissingDirectory(t *testing.T) {
	dir, err := ProfileDir(benchPlatform)
	if err != nil {
		t.Fatalf("profile dir: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove profile dir: %v", err)
	}
	snapshot, err := NewSnapshot(benchPlatform)
	if err != nil {
		t.Fatalf("a missing profile directory must not be an error: %v", err)
	}
	if _, ok := snapshot.FindCached("7656119000000000"); ok {
		t.Fatal("reported a cached image from a directory that does not exist")
	}
	if snapshot.HasManualProfileMarker("7656119000000000") {
		t.Fatal("reported a manual marker from a directory that does not exist")
	}
	if !snapshot.OlderThanDays("7656119000000000", 30) {
		t.Fatal("an account with nothing cached must read as stale")
	}
}

// enrichPerAccount is the shape GetAccountsEnrichment had before the snapshot:
// two independent extension probes plus the manual marker checked twice.
func enrichPerAccount(ids []string, maxAge int) int {
	pendingCount := 0
	for _, id := range ids {
		_, _ = FindCached(benchPlatform, id)
		pending := false
		if HasManualProfileMarker(benchPlatform, id) {
			pending = false
		} else if p, ok := CachedFilePath(benchPlatform, id); ok {
			pending = FileOlderThanDays(p, maxAge)
		} else {
			pending = true
		}
		_ = HasManualProfileMarker(benchPlatform, id)
		if pending {
			pendingCount++
		}
	}
	return pendingCount
}

func enrichSnapshot(ids []string, maxAge int) int {
	snapshot, err := NewSnapshot(benchPlatform)
	if err != nil {
		return -1
	}
	pendingCount := 0
	for _, id := range ids {
		_, _ = snapshot.FindCached(id)
		pending := false
		if !snapshot.HasManualProfileMarker(id) {
			pending = snapshot.OlderThanDays(id, maxAge)
		}
		if pending {
			pendingCount++
		}
	}
	return pendingCount
}

// The benchmark is only meaningful if both shapes classify accounts the same.
func TestEnrichShapesAgree(t *testing.T) {
	ids := seedProfileCache(t, 40)
	if got, want := enrichSnapshot(ids, 30), enrichPerAccount(ids, 30); got != want {
		t.Fatalf("snapshot marked %d accounts pending, per-account marked %d", got, want)
	}
}

// The Steam list asks about three avatar variants per account — primary,
// static and frame — plus the manual marker and the staleness checks behind
// steamAvatarPending. These two model that access pattern through the same
// Lookup interface the real code now uses.
func steamShapedPerAccount(ids []string, maxAge int) int {
	lookup := DirectLookup(benchPlatform)
	return steamShaped(lookup, ids, maxAge)
}

func steamShapedSnapshot(ids []string, maxAge int) int {
	snapshot, err := NewSnapshot(benchPlatform)
	if err != nil {
		return -1
	}
	return steamShaped(snapshot, ids, maxAge)
}

func steamShaped(avatars Lookup, ids []string, maxAge int) int {
	pending := 0
	for _, id := range ids {
		_, _ = avatars.FindCached(id)
		_, _ = avatars.FindCached(id + "_static")
		_, _ = avatars.FindCached(id + "_frame")
		manual := avatars.HasManualProfileMarker(id)
		if manual {
			if avatars.OlderThanDays(id, maxAge) {
				pending++
			}
			continue
		}
		if avatars.OlderThanDays(id+"_static", maxAge) || avatars.OlderThanDays(id, maxAge) {
			pending++
		}
	}
	return pending
}

func TestSteamShapedLookupsAgree(t *testing.T) {
	ids := seedProfileCache(t, 40)
	snapshot, err := NewSnapshot(benchPlatform)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if got, want := steamShaped(snapshot, ids, 30), steamShaped(DirectLookup(benchPlatform), ids, 30); got != want {
		t.Fatalf("snapshot marked %d pending, direct lookup marked %d", got, want)
	}
}

func BenchmarkSteamListAvatarLookup(b *testing.B) {
	for _, accounts := range []int{10, 50, 200} {
		ids := seedProfileCache(b, accounts)
		for _, variant := range []struct {
			name string
			fn   func([]string, int) int
		}{
			{name: "PerAccount", fn: steamShapedPerAccount},
			{name: "Snapshot", fn: steamShapedSnapshot},
		} {
			b.Run(fmt.Sprintf("%daccounts/%s", accounts, variant.name), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if variant.fn(ids, 30) < 0 {
						b.Fatal("snapshot failed")
					}
				}
			})
		}
	}
}

func BenchmarkAccountListAvatarLookup(b *testing.B) {
	for _, accounts := range []int{10, 50, 200} {
		ids := seedProfileCache(b, accounts)
		for _, variant := range []struct {
			name string
			fn   func([]string, int) int
		}{
			{name: "PerAccount", fn: enrichPerAccount},
			{name: "Snapshot", fn: enrichSnapshot},
		} {
			b.Run(fmt.Sprintf("%daccounts/%s", accounts, variant.name), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if variant.fn(ids, 30) < 0 {
						b.Fatal("snapshot failed")
					}
				}
			})
		}
	}
}
