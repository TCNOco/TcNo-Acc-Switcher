package app

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// invalidAppNameChars mirrors the pattern Wails uses to sanitise the app name
// into a GtkApplication id (application_linux.go).
var invalidAppNameChars = regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)

func waylandAppID(name string) string {
	n := invalidAppNameChars.ReplaceAllString(name, "_")
	n = regexp.MustCompile(`^[0-9]`).ReplaceAllString(n, "_$0")
	for strings.Contains(n, "__") {
		n = strings.ReplaceAll(n, "__", "_")
	}
	n = strings.Trim(n, "_")
	if n == "" {
		n = "wailsapp"
	}
	return "org.wails." + strings.ToLower(n)
}

func desktopEntryKey(t *testing.T, key string) string {
	t.Helper()
	path := filepath.Join("..", "..", "build", "linux", "desktop")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return ""
}

// A Wayland compositor matches app_id to a desktop entry to find the window's
// icon, and Wails derives that app_id from the app name, so renaming the app
// silently breaks the match.
func TestDesktopEntryMatchesWaylandAppID(t *testing.T) {
	want := waylandAppID(appName)
	if got := desktopEntryKey(t, "StartupWMClass"); got != want {
		t.Errorf("build/linux/desktop StartupWMClass = %q, want %q (derived from Options.Name %q)", got, want, appName)
	}
}

// Exec must name the binary the packages actually install, or the menu entry
// launches nothing.
func TestDesktopEntryExecMatchesInstalledBinary(t *testing.T) {
	exec := desktopEntryKey(t, "Exec")
	execBin, _, _ := strings.Cut(exec, " ")
	if execBin == "" {
		t.Fatal("build/linux/desktop has no Exec key")
	}
	if strings.ContainsAny(execBin, "/\\") {
		t.Errorf("Exec = %q, want a bare binary name so it resolves on PATH and inside an AppImage", execBin)
	}

	nfpmPath := filepath.Join("..", "..", "build", "linux", "nfpm", "nfpm.yaml")
	raw, err := os.ReadFile(nfpmPath)
	if err != nil {
		t.Fatalf("read %s: %v", nfpmPath, err)
	}
	want := "/usr/local/bin/" + execBin
	if !strings.Contains(string(raw), `dst: "`+want+`"`) {
		t.Errorf("nfpm.yaml installs no binary at %q, which Exec=%q needs", want, execBin)
	}
}

// The Icon key is a theme name, not a path: it only resolves if an icon of that
// name is installed into a hicolor theme directory, which build/linux/icons
// supplies and the nfpm contents install.
func TestDesktopEntryIconHasInstalledTheme(t *testing.T) {
	icon := desktopEntryKey(t, "Icon")
	if icon == "" {
		t.Fatal("build/linux/desktop has no Icon key")
	}
	if strings.ContainsAny(icon, "/\\") || strings.HasSuffix(icon, ".png") {
		t.Errorf("Icon = %q, want a bare theme name", icon)
	}
	root := filepath.Join("..", "..", "build", "linux", "icons")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "apps", icon+".png")); err == nil {
			found++
		}
	}
	if found == 0 {
		t.Errorf("no %s.png under any size in %s", icon, root)
	}
}
