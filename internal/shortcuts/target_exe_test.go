package shortcuts

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

// countingLnkReader replaces the shell resolve, which needs COM and a real
// .lnk, and records how many times it ran.
func countingLnkReader(tb testing.TB, target string, calls *int) {
	tb.Helper()
	previous := readLnkShortcut
	readLnkShortcut = func(string) (string, string, string, error) {
		*calls++
		return target, "", "", nil
	}
	tb.Cleanup(func() { readLnkShortcut = previous })
}

// newShortcutEnv points the login cache at a temp dir and writes one shortcut
// into it. Do NOT call t.Parallel() - it sets global path singletons.
func newShortcutEnv(tb testing.TB, platformKey, fileName string) string {
	tb.Helper()
	paths.ResetForTest(tb.TempDir())
	targetExeCache.Clear()
	tb.Cleanup(func() { targetExeCache.Clear() })

	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		tb.Fatal(err)
	}
	dir := filepath.Join(root, "Shortcuts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatal(err)
	}
	full := filepath.Join(dir, fileName)
	if err := os.WriteFile(full, []byte("lnk"), 0o644); err != nil {
		tb.Fatal(err)
	}
	return full
}

// buildDTOs resolves every shortcut, and runs two or three times per page open
// plus once per change event.
func TestShortcutTargetExeResolvesOncePerShortcut(t *testing.T) {
	calls := 0
	countingLnkReader(t, `C:\Games\Doom\doom.exe`, &calls)
	newShortcutEnv(t, "TestPlatform", "Doom.lnk")

	first := shortcutTargetExe("TestPlatform", "Doom.lnk")
	if first != "doom.exe" {
		t.Fatalf("target = %q, want doom.exe", first)
	}
	if calls != 1 {
		t.Fatalf("first call resolved %d times, want 1", calls)
	}

	for range 3 {
		if got := shortcutTargetExe("TestPlatform", "Doom.lnk"); got != first {
			t.Fatalf("target changed to %q", got)
		}
	}
	if calls != 1 {
		t.Fatalf("resolved %d times across four calls, want 1", calls)
	}
}

// A shortcut repointed at another program has to be picked up.
func TestShortcutTargetExeReResolvesWhenTheShortcutChanges(t *testing.T) {
	calls := 0
	countingLnkReader(t, `C:\Games\Doom\doom.exe`, &calls)
	full := newShortcutEnv(t, "TestPlatform", "Doom.lnk")

	if got := shortcutTargetExe("TestPlatform", "Doom.lnk"); got != "doom.exe" {
		t.Fatalf("target = %q", got)
	}

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(full, later, later); err != nil {
		t.Fatal(err)
	}
	countingLnkReader(t, `C:\Games\Quake\quake.exe`, &calls)

	if got := shortcutTargetExe("TestPlatform", "Doom.lnk"); got != "quake.exe" {
		t.Fatalf("target = %q, want quake.exe after the shortcut changed", got)
	}
	if calls != 2 {
		t.Fatalf("resolved %d times, want 2", calls)
	}
}

// A .lnk pointing at something that is not a program resolves to "" - and that
// answer has to be remembered too, not re-resolved every time.
func TestShortcutTargetExeRemembersANonProgramTarget(t *testing.T) {
	calls := 0
	countingLnkReader(t, `C:\Users\someone\Documents`, &calls)
	newShortcutEnv(t, "TestPlatform", "Docs.lnk")

	for range 3 {
		if got := shortcutTargetExe("TestPlatform", "Docs.lnk"); got != "" {
			t.Fatalf("target = %q, want empty", got)
		}
	}
	if calls != 1 {
		t.Fatalf("resolved %d times, want 1", calls)
	}
}
