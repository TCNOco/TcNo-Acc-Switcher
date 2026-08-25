package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

// benchStartupAccounts is a well-used install's worth of accounts on each of the
// platforms the user actually has.
const benchStartupAccounts = 15

// seedStartupEnv builds an install the startup snapshot has real work to do on:
// the shipped 24-platform catalog, and an ids.json for each of them.
func seedStartupEnv(tb testing.TB) {
	tb.Helper()
	exeDir := tb.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))

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
		dir, err := paths.LoginCacheDir(name)
		if err != nil {
			tb.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatal(err)
		}
		ids := make(map[string]string, benchStartupAccounts)
		lastUsed := make(map[string]string, benchStartupAccounts)
		for i := range benchStartupAccounts {
			uid := fmt.Sprintf("%s-account-%02d", name, i)
			ids[uid] = fmt.Sprintf("Account %d", i)
			lastUsed[uid] = "2026-01-01T00:00:00Z"
		}
		body, err := json.Marshal(map[string]any{"ids": ids, "lastused": lastUsed})
		if err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ids.json"), body, 0o644); err != nil {
			tb.Fatal(err)
		}
	}

	RegisterStartupAccountCounts()
}

// BenchmarkGetStartup is the full startup payload the home screen asks for,
// including the per-platform account and tag totals it paints as skeleton hints.
func BenchmarkGetStartup(b *testing.B) {
	seedStartupEnv(b)
	svc := &platform.PlatformService{}
	if _, err := svc.GetStartup(); err != nil {
		b.Fatalf("GetStartup: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := svc.GetStartup(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadSettings is the same payload without those counts. Two of the
// four startup-snapshot calls a cold start makes come through here, and both
// want a single scalar setting.
func BenchmarkReadSettings(b *testing.B) {
	seedStartupEnv(b)
	svc := &platform.PlatformService{}
	if _, err := svc.ReadSettings(); err != nil {
		b.Fatalf("ReadSettings: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := svc.ReadSettings(); err != nil {
			b.Fatal(err)
		}
	}
}
