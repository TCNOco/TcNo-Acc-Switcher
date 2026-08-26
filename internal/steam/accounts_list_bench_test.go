package steam

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
)

// benchSteamEnv mirrors newSteamTestEnv for benchmarks. Do NOT run these in
// parallel - they set global path singletons.
func benchSteamEnv(tb testing.TB) string {
	tb.Helper()
	exeDir := tb.TempDir()
	steamDir := tb.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
	if err := os.MkdirAll(filepath.Join(steamDir, "config"), 0o755); err != nil {
		tb.Fatal(err)
	}
	userData := platform.UserDataDir(exeDir)
	if err := os.MkdirAll(userData, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, platform.PlatformsFileName()),
		[]byte(`{"Version":"test","Platforms":{"Steam":{"ExeLocationDefault":[]}}}`), 0o644); err != nil {
		tb.Fatal(err)
	}
	st, err := LoadSettings()
	if err != nil {
		tb.Fatalf("LoadSettings: %v", err)
	}
	st.FolderPath = steamDir
	st.SteamShowMiniProfile = true
	if err := SaveSettings(st); err != nil {
		tb.Fatalf("SaveSettings: %v", err)
	}
	return steamDir
}

// seedSteamAccounts writes a loginusers.vdf with n accounts plus the cached
// miniprofile HTML and avatar files the enrichment pass reads per account.
func seedSteamAccounts(tb testing.TB, steamDir string, n int) []string {
	tb.Helper()

	var vdf strings.Builder
	vdf.WriteString("\"users\"\n{\n")
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("765611980000%05d", i)
		ids = append(ids, id)
		fmt.Fprintf(&vdf, "\t%q\n\t{\n\t\t\"AccountName\"\t\t\"acct_%d\"\n\t\t\"PersonaName\"\t\t\"Persona %d\"\n\t\t\"Timestamp\"\t\t\"1700000000\"\n\t}\n", id, i, i)
	}
	vdf.WriteString("}\n")
	if err := os.WriteFile(filepath.Join(steamDir, "config", "loginusers.vdf"), []byte(vdf.String()), 0o644); err != nil {
		tb.Fatal(err)
	}

	cacheRoot, err := paths.LoginCacheDir(PlatformKey)
	if err != nil {
		tb.Fatal(err)
	}
	miniDir := filepath.Join(cacheRoot, "MiniProfileCache")
	if err := os.MkdirAll(miniDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	profDir, err := profileimage.ProfileDir(PlatformKey)
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	for i, id := range ids {
		html := fmt.Sprintf(`<div class="miniprofile_container"><div class="playersection_avatar"><img src="https://avatars.example/%s.jpg"></div><span class="persona online">Persona %d</span><div class="miniprofile_gamename">Some Game</div></div>`, id, i)
		if err := os.WriteFile(filepath.Join(miniDir, id+".html"), []byte(html), 0o644); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(profDir, id+".jpg"), []byte("jpegbytes"), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return ids
}

func benchSteamSizes() []int { return []int{10, 50, 200} }

// BenchmarkBuildSteamListContext measures the shared setup both
// GetSteamAccountsList and GetSteamAccountsEnrichment run, so opening a Steam
// account page pays it twice.
func BenchmarkBuildSteamListContext(b *testing.B) {
	for _, n := range benchSteamSizes() {
		b.Run(fmt.Sprintf("%daccounts", n), func(b *testing.B) {
			steamDir := benchSteamEnv(b)
			seedSteamAccounts(b, steamDir, n)
			svc := &SteamService{}
			if _, err := svc.buildSteamListContext(); err != nil {
				b.Fatalf("buildSteamListContext: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.buildSteamListContext(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSteamEnrichment is the per-account pass: cached miniprofile HTML,
// avatar URLs, bans, tags and notes for every account on the page.
func BenchmarkSteamEnrichment(b *testing.B) {
	for _, n := range benchSteamSizes() {
		b.Run(fmt.Sprintf("%daccounts", n), func(b *testing.B) {
			steamDir := benchSteamEnv(b)
			seedSteamAccounts(b, steamDir, n)
			svc := &SteamService{}
			got, err := svc.GetSteamAccountsEnrichment()
			if err != nil {
				b.Fatalf("GetSteamAccountsEnrichment: %v", err)
			}
			if len(got) != n {
				b.Fatalf("enrichment returned %d accounts, want %d", len(got), n)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.GetSteamAccountsEnrichment(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// seedTrayUsers writes Tray_Users.json with n Steam entries.
func seedTrayUsers(tb testing.TB, ids []string, n int) {
	tb.Helper()
	root, err := paths.DataRoot()
	if err != nil {
		tb.Fatal(err)
	}
	entries := make([]map[string]string, 0, n)
	for i := 0; i < n && i < len(ids); i++ {
		entries = append(entries, map[string]string{"Name": "Account", "Arg": "+s:" + ids[i]})
	}
	body, err := json.Marshal(map[string][]map[string]string{PlatformKey: entries})
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Tray_Users.json"), body, 0o644); err != nil {
		tb.Fatal(err)
	}
}

// BenchmarkSyncTrayKnownAccounts is the Steam tray prune that runs before the
// window is created; the tray keeps three accounts however many the install has.
func BenchmarkSyncTrayKnownAccounts(b *testing.B) {
	for _, n := range []int{50, 200} {
		b.Run(fmt.Sprintf("%daccounts", n), func(b *testing.B) {
			steamDir := benchSteamEnv(b)
			ids := seedSteamAccounts(b, steamDir, n)
			seedTrayUsers(b, ids, 3)

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				SyncTrayKnownAccounts()
			}
		})
	}
}

// BenchmarkDropStaleMiniprofileFragments is the staleness sweep the profile
// refresh runs per account. Each account asks about five avatar variants, and
// probing the filesystem walks the extension list for every one of them.
func BenchmarkDropStaleMiniprofileFragments(b *testing.B) {
	for _, n := range []int{50, 200} {
		steamDir := benchSteamEnv(b)
		ids := seedSteamAccounts(b, steamDir, n)

		b.Run(fmt.Sprintf("%daccounts/PerAccount", n), func(b *testing.B) {
			direct := profileimage.DirectLookup(PlatformKey)
			b.ReportAllocs()
			for b.Loop() {
				for _, id := range ids {
					dropStaleMiniprofileFragment(direct, id, 7)
				}
			}
		})

		b.Run(fmt.Sprintf("%daccounts/Snapshot", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				snap, err := profileimage.NewSnapshot(PlatformKey)
				if err != nil {
					b.Fatal(err)
				}
				for _, id := range ids {
					dropStaleMiniprofileFragment(snap, id, 7)
				}
			}
		})
	}
}
