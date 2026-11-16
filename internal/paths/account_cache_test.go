package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAccountCacheDirFindsLegacyName(t *testing.T) {
	ResetForTest(t.TempDir())
	base, err := LoginCacheDir("BattleNet")
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(base, "Player#1234")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := AccountCacheDir("BattleNet", "Player#1234")
	if err != nil || got != legacy {
		t.Fatalf("cache = %q, %v", got, err)
	}
	if err := os.MkdirAll(filepath.Join(base, "Player_1234"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AccountCacheDir("BattleNet", "Player#1234"); err == nil {
		t.Fatal("ambiguous cache directories accepted")
	}
}

func TestAccountCacheDirUsesNormalizedNameForNewAccounts(t *testing.T) {
	ResetForTest(t.TempDir())
	base, err := LoginCacheDir("BattleNet")
	if err != nil {
		t.Fatal(err)
	}
	got, err := AccountCacheDir("BattleNet", "Player#1234")
	if err != nil || got != filepath.Join(base, "Player_1234") {
		t.Fatalf("cache = %q, %v", got, err)
	}
}

func TestSafeLegacyAccountNameRejectsPaths(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../outside", `..\outside`, `C:\outside`, "account.", "account ", "NUL", "CON.txt", "a\x00b", "a:b"} {
		if safeLegacyAccountName(name) {
			t.Errorf("unsafe legacy name accepted: %q", name)
		}
	}
	for _, name := range []string{"Player#1234", "player@example.test", "Player 🎮"} {
		if !safeLegacyAccountName(name) {
			t.Errorf("valid legacy name rejected: %q", name)
		}
	}
}
