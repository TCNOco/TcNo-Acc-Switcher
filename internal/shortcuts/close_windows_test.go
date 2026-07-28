//go:build windows

package shortcuts

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/winutil"
)

// Closing is offered per shortcut, so what a shortcut points at decides whether
// the option appears at all: a link to a program can be closed, a link to a web
// page or a document leaves no process behind.
func TestShortcutTargetExe(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	const platformKey = "TestPlatform"
	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "Shortcuts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	exe := filepath.Join(t.TempDir(), "Assetto Corsa.exe")
	if err := os.WriteFile(exe, []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	game := filepath.Join(dir, "Assetto Corsa.lnk")
	if err := winutil.WriteShortcutLnk(game, exe, "", filepath.Dir(exe), "", "", ""); err != nil {
		t.Skipf("shortcuts cannot be written here: %v", err)
	}

	folder := filepath.Join(dir, "Screenshots.lnk")
	if err := winutil.WriteShortcutLnk(folder, filepath.Dir(exe), "", "", "", "", ""); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "Store Page.url")
	if err := os.WriteFile(link, []byte("[InternetShortcut]\r\nURL=steam://run/244210\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := shortcutTargetExe(platformKey, "Assetto Corsa.lnk"); got != "Assetto Corsa.exe" {
		t.Fatalf("program shortcut target = %q, want Assetto Corsa.exe", got)
	}
	if got := shortcutTargetExe(platformKey, "Screenshots.lnk"); got != "" {
		t.Fatalf("folder shortcut reported a program: %q", got)
	}
	if got := shortcutTargetExe(platformKey, "Store Page.url"); got != "" {
		t.Fatalf("url shortcut reported a program: %q", got)
	}
	if got := shortcutTargetExe(platformKey, "Missing.lnk"); got != "" {
		t.Fatalf("missing shortcut reported a program: %q", got)
	}
}

// A shortcut with nothing to close must say so rather than killing whatever
// process the name happens to resemble.
func TestCloseShortcutRejectsNonProgram(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	const platformKey = "TestPlatform"
	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "Shortcuts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "Store Page.url")
	if err := os.WriteFile(link, []byte("[InternetShortcut]\r\nURL=https://example.com\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := &Service{scanBusy: map[string]struct{}{}}
	if err := service.CloseShortcut(platformKey, "Store Page.url"); err == nil {
		t.Fatal("closing a web shortcut was accepted")
	}
	if err := service.CloseShortcut(platformKey, ""); err == nil {
		t.Fatal("closing an unnamed shortcut was accepted")
	}
}
