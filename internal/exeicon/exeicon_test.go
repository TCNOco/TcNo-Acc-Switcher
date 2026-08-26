package exeicon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// countingExtractor replaces the real icon extraction, which needs the shell and
// a real shortcut.
func countingExtractor(tb testing.TB, calls *int) {
	tb.Helper()
	previousShortcut, previousExe := extractShortcutIcon, extractExeIcon
	stub := func(_, outPNG string) error {
		*calls++
		if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
			return err
		}
		return os.WriteFile(outPNG, []byte("png"), 0o644)
	}
	extractShortcutIcon, extractExeIcon = stub, stub
	tb.Cleanup(func() { extractShortcutIcon, extractExeIcon = previousShortcut, previousExe })
}

func writeSource(tb testing.TB, path string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("source"), 0o644); err != nil {
		tb.Fatal(err)
	}
}

// The account page asks for the platform icon on every mount.
func TestEnsureShortcutCachedExtractsOnlyOnce(t *testing.T) {
	calls := 0
	countingExtractor(t, &calls)
	root := t.TempDir()
	shortcut := filepath.Join(root, "Discord.lnk")
	writeSource(t, shortcut)
	www := filepath.Join(root, "wwwroot")

	first, err := EnsureShortcutCached("Discord", "Discord.exe", shortcut, www)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("first call extracted %d times, want 1", calls)
	}

	second, err := EnsureShortcutCached("Discord", "Discord.exe", shortcut, www)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("second call extracted again (%d total); the cached PNG should have been used", calls)
	}
	if first != second {
		t.Fatalf("URL changed between calls: %q then %q", first, second)
	}
}

// A shortcut that has been repointed must still refresh the icon.
func TestEnsureShortcutCachedReExtractsWhenTheShortcutIsNewer(t *testing.T) {
	calls := 0
	countingExtractor(t, &calls)
	root := t.TempDir()
	shortcut := filepath.Join(root, "Discord.lnk")
	writeSource(t, shortcut)
	www := filepath.Join(root, "wwwroot")

	if _, err := EnsureShortcutCached("Discord", "Discord.exe", shortcut, www); err != nil {
		t.Fatalf("first call: %v", err)
	}

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(shortcut, later, later); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureShortcutCached("Discord", "Discord.exe", shortcut, www); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 2 {
		t.Fatalf("extracted %d times, want 2 - a newer shortcut must refresh the icon", calls)
	}
}

func TestEnsureCachedExtractsOnlyOnce(t *testing.T) {
	calls := 0
	countingExtractor(t, &calls)
	root := t.TempDir()
	exe := filepath.Join(root, "Discord.exe")
	writeSource(t, exe)
	www := filepath.Join(root, "wwwroot")

	if _, err := EnsureCached("Discord", exe, www); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := EnsureCached("Discord", exe, www); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("extracted %d times, want 1", calls)
	}
}
