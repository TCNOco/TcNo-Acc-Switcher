//go:build windows

package shortcuts

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/winutil"
)

func writeTestShortcut(t *testing.T, lnk, target, args string) {
	t.Helper()
	if err := winutil.WriteShortcutLnk(lnk, target, args, filepath.Dir(target), "test", "", ""); err != nil {
		t.Fatalf("WriteShortcutLnk: %v", err)
	}
}

func TestShortcutAlreadyMatches(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	lnk := filepath.Join(dir, "existing.lnk")
	writeTestShortcut(t, lnk, exe, "+s:76561197960287930")

	t.Run("same target and arguments", func(t *testing.T) {
		if !shortcutAlreadyMatches(lnk, exe, "+s:76561197960287930") {
			t.Error("an identical shortcut was not recognised, so the user gets told one was created")
		}
	})

	// The arguments carry the account, so a different account behind the same
	// filename must rewrite rather than report the old one as already correct.
	t.Run("different arguments", func(t *testing.T) {
		if shortcutAlreadyMatches(lnk, exe, "+s:76561197960287931") {
			t.Error("a shortcut for a different account was treated as a match")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if shortcutAlreadyMatches(filepath.Join(dir, "absent.lnk"), exe, "+s:76561197960287930") {
			t.Error("a nonexistent shortcut was reported as already there")
		}
	})

	t.Run("directory in the way", func(t *testing.T) {
		asDir := filepath.Join(dir, "dir.lnk")
		if err := os.Mkdir(asDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if shortcutAlreadyMatches(asDir, exe, "+s:76561197960287930") {
			t.Error("a directory was treated as a matching shortcut")
		}
	})
}
