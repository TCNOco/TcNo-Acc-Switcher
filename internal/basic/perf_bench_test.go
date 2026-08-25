package basic

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

// benchPlatformCount and benchAccountsPerPlatform model a well-used install:
// Platforms.json ships 24 platforms, and the startup hint pass touches every one
// of them regardless of how many the user actually has accounts for.
const (
	benchPlatformCount       = 24
	benchAccountsPerPlatform = 15
	benchTagsPerPlatform     = 6
)

func benchResetPaths(tb testing.TB) {
	tb.Helper()
	exeDir := tb.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
}

// seedBenchPlatforms writes a realistic ids.json per platform: accounts, tags,
// tag assignments and per-account expiries, so the parse and prune work in the
// benchmark matches what a real install pays for.
func seedBenchPlatforms(tb testing.TB, platforms, accounts int) []string {
	tb.Helper()
	future := time.Now().UTC().Add(90 * 24 * time.Hour).Format(time.RFC3339)

	names := make([]string, 0, platforms)
	for p := 0; p < platforms; p++ {
		name := fmt.Sprintf("BenchPlatform%02d", p)
		names = append(names, name)

		f := idsFile{
			IDs:                make(map[string]string, accounts),
			LastUsed:           make(map[string]string, accounts),
			Tags:               make(map[string]tagFileEntry, benchTagsPerPlatform),
			AccountTags:        make(map[string][]string, accounts),
			AccountTagExpiries: make(map[string]map[string]string, accounts),
		}
		for t := 0; t < benchTagsPerPlatform; t++ {
			f.Tags[fmt.Sprintf("tag-%d", t)] = tagFileEntry{
				Name:  fmt.Sprintf("Tag %d", t),
				Color: "#3366cc",
			}
		}
		for a := 0; a < accounts; a++ {
			uid := fmt.Sprintf("7656119%09d", p*1000+a)
			f.IDs[uid] = fmt.Sprintf("Account %d of %s", a, name)
			f.LastUsed[uid] = future
			assigned := []string{
				fmt.Sprintf("tag-%d", a%benchTagsPerPlatform),
				fmt.Sprintf("tag-%d", (a+1)%benchTagsPerPlatform),
			}
			f.AccountTags[uid] = assigned
			f.AccountTagExpiries[uid] = map[string]string{assigned[0]: future}
		}
		if err := writeIdsFile(name, f); err != nil {
			tb.Fatalf("seed %s: %v", name, err)
		}
		if err := writeOrder(name, nil); err != nil {
			tb.Fatalf("seed order %s: %v", name, err)
		}
	}
	return names
}

// legacyStartupCounts is the shipped-before pass: three separate ids.json reads
// per platform, one per count, run serially. Kept here as the benchmark baseline
// the combined and parallel variants below are measured against.
func legacyStartupCounts(names []string) (map[string]int, map[string][2]int) {
	accounts := make(map[string]int, len(names))
	tags := make(map[string][2]int, len(names))
	for _, n := range names {
		f, err := readIdsFile(n)
		if err != nil {
			continue
		}
		accounts[n] = len(f.IDs)
	}
	for _, n := range names {
		f1, err := readIdsFile(n)
		if err != nil {
			continue
		}
		f2, err := readIdsFile(n)
		if err != nil {
			continue
		}
		tags[n] = [2]int{len(f1.Tags), len(f2.AccountTags)}
	}
	return accounts, tags
}

// BenchmarkStartupCountsSeparate is the per-platform count pass GetStartup ran
// before this change: three reads and parses of the same file per platform.
// It happens before the window is drawn, so it is on the cold-start path.
func BenchmarkStartupCountsSeparate(b *testing.B) {
	benchResetPaths(b)
	names := seedBenchPlatforms(b, benchPlatformCount, benchAccountsPerPlatform)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		legacyStartupCounts(names)
	}
}

// BenchmarkStartupCountsCombined reads each platform's ids.json once.
func BenchmarkStartupCountsCombined(b *testing.B) {
	benchResetPaths(b)
	names := seedBenchPlatforms(b, benchPlatformCount, benchAccountsPerPlatform)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, n := range names {
			_ = CountsFor(n)
		}
	}
}

// BenchmarkStartupCountsCombinedParallel mirrors the fan-out the real resolver
// in internal/app uses: one read per platform, platforms resolved concurrently.
func BenchmarkStartupCountsCombinedParallel(b *testing.B) {
	benchResetPaths(b)
	names := seedBenchPlatforms(b, benchPlatformCount, benchAccountsPerPlatform)

	const workers = 8
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := make([]PlatformCounts, len(names))
		var next atomic.Int64
		var wg sync.WaitGroup
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func() {
				defer wg.Done()
				for {
					j := int(next.Add(1)) - 1
					if j >= len(names) {
						return
					}
					out[j] = CountsFor(names[j])
				}
			}()
		}
		wg.Wait()
	}
}

// BenchmarkReadIdsFile isolates the single-file cost the count pass multiplies.
func BenchmarkReadIdsFile(b *testing.B) {
	benchResetPaths(b)
	names := seedBenchPlatforms(b, 1, benchAccountsPerPlatform)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := readIdsFile(names[0]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBuildAccountTagMap covers the tag map the account page requests
// alongside the list itself.
func BenchmarkBuildAccountTagMap(b *testing.B) {
	benchResetPaths(b)
	names := seedBenchPlatforms(b, 1, benchAccountsPerPlatform)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BuildAccountTagMap(names[0]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolveTagsForAllAccounts is the per-row tag resolution the account
// list and the enrichment pass each run once over every account.
func BenchmarkResolveTagsForAllAccounts(b *testing.B) {
	benchResetPaths(b)
	names := seedBenchPlatforms(b, 1, benchAccountsPerPlatform)
	f, err := readIdsFile(names[0])
	if err != nil {
		b.Fatal(err)
	}
	uids := make([]string, 0, len(f.IDs))
	for uid := range f.IDs {
		uids = append(uids, uid)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, uid := range uids {
			_ = resolveTagsForAccount(f, uid)
		}
	}
}
