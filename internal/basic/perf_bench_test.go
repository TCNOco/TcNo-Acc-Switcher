package basic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/security"
)

// benchPlatformCount matches the 24 platforms Platforms.json ships: the startup
// pass touches every one regardless of how many the user has accounts for. The
// per-platform account and tag counts model a well-used install, not a fresh one.
const (
	benchPlatformCount       = 24
	benchAccountsPerPlatform = 15
	benchTagsPerPlatform     = 6
)

func benchResetPaths(tb testing.TB) string {
	tb.Helper()
	exeDir := tb.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
	return exeDir
}

// seedBenchCatalog writes the real shipped Platforms.json with the benchmark's
// platforms added to it. Without a catalog buildAccountListContext silently skips
// the descriptor parse and the live-unique-id read; the catalog keeps its real
// size because the descriptor parse cost scales with it.
func seedBenchCatalog(tb testing.TB, exeDir string, platformNames []string) {
	tb.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "Platforms.json"))
	if err != nil {
		tb.Skipf("Platforms.json unavailable: %v", err)
	}
	var top struct {
		Version   string                     `json:"Version"`
		Platforms map[string]json.RawMessage `json:"Platforms"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		tb.Fatalf("parse catalog: %v", err)
	}

	idFile := filepath.Join(tb.TempDir(), ".account_id")
	if err := os.WriteFile(idFile, []byte("live-unique-id"), 0o644); err != nil {
		tb.Fatal(err)
	}
	entry, err := json.Marshal(map[string]any{
		"ExeLocationDefault": []string{},
		"UniqueIdMethod":     "CREATE_ID_FILE",
		"UniqueIdFile":       idFile,
		"LoginFiles":         map[string]string{`%Platform_Folder%\config.cfg`: "Saved/config.cfg"},
	})
	if err != nil {
		tb.Fatal(err)
	}
	for _, n := range platformNames {
		top.Platforms[n] = entry
	}

	out, err := json.Marshal(top)
	if err != nil {
		tb.Fatal(err)
	}
	dir := platform.UserDataDir(exeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, platform.PlatformsFileName()), out, 0o644); err != nil {
		tb.Fatal(err)
	}
}

// seedBenchPlatforms writes an ids.json per platform with tags, assignments and
// expiries, so the parse and prune work matches what a real install pays for.
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

// legacyStartupCounts is the baseline the combined and parallel variants are
// measured against: three separate ids.json reads per platform, run serially.
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

// BenchmarkStartupCountsSeparate is the per-platform count pass at three reads
// of the same file each. It runs before the window is drawn, so it is cold-start
// cost.
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

// seedBenchAvatars writes a cached avatar per account so the enrichment pass
// resolves real URLs rather than short-circuiting on an empty directory.
func seedBenchAvatars(tb testing.TB, platformKey string, uids []string) {
	tb.Helper()
	dir, err := profileimage.ProfileDir(platformKey)
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	for _, uid := range uids {
		if err := os.WriteFile(filepath.Join(dir, uid+".jpg"), []byte("jpegbytes"), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
}

func benchAccountSizes() []int { return []int{10, 50, 200} }

// BenchmarkGetAccountsList is the fast payload the account page paints first.
func BenchmarkGetAccountsList(b *testing.B) {
	for _, n := range benchAccountSizes() {
		b.Run(fmt.Sprintf("%daccounts", n), func(b *testing.B) {
			benchResetPaths(b)
			names := seedBenchPlatforms(b, 1, n)
			svc := &BasicService{}
			if _, err := svc.GetAccountsList(names[0]); err != nil {
				b.Fatalf("GetAccountsList: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.GetAccountsList(names[0]); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGetAccountsEnrichment is the slower second payload: avatars, notes,
// tags and last-used for every account on the page.
func BenchmarkGetAccountsEnrichment(b *testing.B) {
	for _, n := range benchAccountSizes() {
		b.Run(fmt.Sprintf("%daccounts", n), func(b *testing.B) {
			benchResetPaths(b)
			names := seedBenchPlatforms(b, 1, n)
			f, err := readIdsFile(names[0])
			if err != nil {
				b.Fatal(err)
			}
			uids := make([]string, 0, len(f.IDs))
			for uid := range f.IDs {
				uids = append(uids, uid)
			}
			seedBenchAvatars(b, names[0], uids)

			svc := &BasicService{}
			got, err := svc.GetAccountsEnrichment(names[0])
			if err != nil {
				b.Fatalf("GetAccountsEnrichment: %v", err)
			}
			if len(got) != n {
				b.Fatalf("enrichment returned %d rows, want %d", len(got), n)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.GetAccountsEnrichment(names[0]); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// benchVaultPassword only ever protects a temp directory created by the
// benchmark below.
const benchVaultPassword = "bench vault password Aa1!"

// seedEncryptedAccounts writes one encrypted blob per account so SavedDataBroken
// has real work to verify. payloadKB is each account's plaintext payload size.
func seedEncryptedAccounts(tb testing.TB, platformKey string, uids []string, payloadKB int) {
	tb.Helper()
	if err := security.SetAppPassword(benchVaultPassword); err != nil {
		tb.Skipf("cannot set app password: %v", err)
	}
	if err := security.EnableSavedAccountEncryption(benchVaultPassword); err != nil {
		tb.Skipf("cannot enable saved-account encryption: %v", err)
	}
	tb.Cleanup(func() {
		_ = security.DisableSavedAccountEncryption(benchVaultPassword)
		_ = security.RemoveAppPassword(benchVaultPassword)
	})

	payload := bytes.Repeat([]byte("saved account login payload blob\n"), payloadKB*1024/33)
	for _, uid := range uids {
		save, err := security.BeginAccountSave(platformKey, uid, uid, "")
		if err != nil {
			tb.Fatalf("BeginAccountSave: %v", err)
		}
		if err := os.WriteFile(filepath.Join(save.DestRoot, "config.cfg"), payload, 0o644); err != nil {
			security.CleanupAccountSave(save)
			tb.Fatalf("write payload: %v", err)
		}
		if err := security.CommitAccountSave(save, ""); err != nil {
			tb.Fatalf("CommitAccountSave: %v", err)
		}
	}
	if !security.SavedAccountDataEncrypted() {
		tb.Skip("saved-account encryption did not stay enabled")
	}
}

// BenchmarkGetAccountsListEncrypted is the same first payload on an install
// with saved-account encryption on. Every row's SavedDataBroken flag costs a
// full read and AEAD-decrypt of that account's blob.
func BenchmarkGetAccountsListEncrypted(b *testing.B) {
	for _, n := range []int{10, 50} {
		b.Run(fmt.Sprintf("%daccounts", n), func(b *testing.B) {
			benchResetPaths(b)
			names := seedBenchPlatforms(b, 1, n)
			f, err := readIdsFile(names[0])
			if err != nil {
				b.Fatal(err)
			}
			uids := make([]string, 0, len(f.IDs))
			for uid := range f.IDs {
				uids = append(uids, uid)
			}
			seedEncryptedAccounts(b, names[0], uids, 64)

			svc := &BasicService{}
			if _, err := svc.GetAccountsList(names[0]); err != nil {
				b.Fatalf("GetAccountsList: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.GetAccountsList(names[0]); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGetAccountsListWithCatalog is GetAccountsList on the path a real
// install takes: a populated Platforms.json, so the descriptor parse and the
// live-unique-id read both happen (the plain variant above skips both).
func BenchmarkGetAccountsListWithCatalog(b *testing.B) {
	for _, n := range benchAccountSizes() {
		b.Run(fmt.Sprintf("%daccounts", n), func(b *testing.B) {
			exeDir := benchResetPaths(b)
			names := seedBenchPlatforms(b, 1, n)
			seedBenchCatalog(b, exeDir, names)

			svc := &BasicService{}
			got, err := svc.GetAccountsList(names[0])
			if err != nil {
				b.Fatalf("GetAccountsList: %v", err)
			}
			if len(got) != n {
				b.Fatalf("list returned %d rows, want %d", len(got), n)
			}
			if !got[0].CurrentSession && !got[len(got)-1].CurrentSession {
				// The descriptor path is what makes this benchmark
				// different - warn if it did not engage.
				b.Logf("note: no row matched the live unique id")
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.GetAccountsList(names[0]); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// seedTrayBenchEnv gives every catalog platform an ids.json but tray entries for
// only one of them, which is what most installs look like.
func seedTrayBenchEnv(tb testing.TB, trayPlatform string) {
	tb.Helper()
	exeDir := benchResetPaths(tb)

	raw, err := os.ReadFile(filepath.Join("..", "..", "Platforms.json"))
	if err != nil {
		tb.Skipf("Platforms.json unavailable: %v", err)
	}
	userData := platform.UserDataDir(exeDir)
	if err := os.MkdirAll(userData, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, platform.PlatformsFileName()), raw, 0o644); err != nil {
		tb.Fatal(err)
	}

	var top struct {
		Platforms map[string]json.RawMessage `json:"Platforms"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		tb.Fatalf("parse catalog: %v", err)
	}
	for name := range top.Platforms {
		f := idsFile{IDs: map[string]string{}, LastUsed: map[string]string{}}
		for i := range benchAccountsPerPlatform {
			uid := fmt.Sprintf("%s-account-%02d", name, i)
			f.IDs[uid] = fmt.Sprintf("Account %d", i)
		}
		if err := writeIdsFile(name, f); err != nil {
			tb.Fatalf("seed %s: %v", name, err)
		}
	}

	root, err := paths.DataRoot()
	if err != nil {
		tb.Fatal(err)
	}
	trayBody, err := json.Marshal(map[string][]map[string]string{
		trayPlatform: {{"Name": "Account 0", "Arg": "+x:" + trayPlatform + "-account-00"}},
	})
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Tray_Users.json"), trayBody, 0o644); err != nil {
		tb.Fatalf("seed tray users: %v", err)
	}
}

// BenchmarkSyncAllTrayKnownAccounts is the tray prune that runs before the
// window is created.
func BenchmarkSyncAllTrayKnownAccounts(b *testing.B) {
	seedTrayBenchEnv(b, "Discord")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		SyncAllTrayKnownAccounts()
	}
}

// BenchmarkGameStatsMarkupNoStatsConfigured is the account page asking for game
// stats on a platform that has none, which is most of the ones the catalog ships.
func BenchmarkGameStatsMarkupNoStatsConfigured(b *testing.B) {
	exeDir := benchResetPaths(b)
	names := seedBenchPlatforms(b, 1, 10)
	seedBenchCatalog(b, exeDir, names)

	// The real definitions, so the lookup misses for the same reason it does on a
	// real install: nothing is configured for this platform.
	gameStats, err := os.ReadFile(filepath.Join("..", "..", "GameStats.json"))
	if err != nil {
		b.Skipf("GameStats.json unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(exeDir, "GameStats.json"), gameStats, 0o644); err != nil {
		b.Fatal(err)
	}

	svc := &BasicService{}
	got, gerr := svc.GetUserStatsAllGamesMarkup(names[0], "7656119000000000")
	if gerr != nil {
		b.Fatalf("GetUserStatsAllGamesMarkup: %v", gerr)
	}
	if len(got) != 0 {
		b.Fatalf("expected no stats for an unconfigured platform, got %d", len(got))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := svc.GetUserStatsAllGamesMarkup(names[0], "7656119000000000"); err != nil {
			b.Fatal(err)
		}
	}
}
