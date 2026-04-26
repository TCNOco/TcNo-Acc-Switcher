package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearCachePathLeavesDirectoryAndRemovesContents(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "cache")
	if err := os.MkdirAll(filepath.Join(cacheDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "file.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "nested", "file.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := clearCachePath(cacheDir); err != nil {
		t.Fatal(err)
	}

	if st, err := os.Stat(cacheDir); err != nil || !st.IsDir() {
		t.Fatalf("cache directory should remain, stat=%v err=%v", st, err)
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cache directory should be empty, got %d entries", len(entries))
	}
}

func TestClearCachePathDeletesFile(t *testing.T) {
	root := t.TempDir()
	cacheFile := filepath.Join(root, "cache.tmp")
	if err := os.WriteFile(cacheFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := clearCachePath(cacheFile); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatalf("cache file should be deleted, err=%v", err)
	}
}

func TestResolveSafeDeletePatternRejectsPlaceholderBase(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "Desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", root)

	if _, err := ResolveSafeDeletePattern("%Desktop%", PathTokenContext{}); err == nil {
		t.Fatal("expected placeholder base path to be rejected")
	}
}

func TestResolveSafeDeletePatternRejectsGlobAtPlaceholderBase(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "Desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", root)

	if _, err := ResolveSafeDeletePattern(`%Desktop%\*`, PathTokenContext{}); err == nil {
		t.Fatal("expected glob directly under placeholder base to be rejected")
	}
}

func TestResolveSafeDeletePatternRejectsUnresolvedPlaceholder(t *testing.T) {
	if _, err := ResolveSafeDeletePattern(`%DefinitelyMissing%\cache`, PathTokenContext{}); err == nil {
		t.Fatal("expected unresolved placeholder to be rejected")
	}
}

func TestResolveSafeDeletePatternAllowsMissingTargetBelowPlaceholder(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "Desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", root)

	got, err := ResolveSafeDeletePattern(`%Desktop%\missing-cache`, PathTokenContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got == "" || samePath(got, desktop) {
		t.Fatalf("expected path below placeholder base, got %q", got)
	}
}
