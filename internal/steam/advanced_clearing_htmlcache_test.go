//go:build !windows

package steam

import (
	"os"
	"path/filepath"
	"testing"
)

// The cache belongs to whichever install the user resolved, so it has to come
// off that root rather than off $HOME, which on a Flatpak machine names a
// different install or a path that does not exist.
func TestHTMLCachePathFollowsTheResolvedRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	flatpak := filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam")
	want := filepath.Join(flatpak, "config", "htmlcache")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	// A native install alongside it, which a $HOME-derived path would pick.
	native := filepath.Join(home, ".local", "share", "Steam", "config", "htmlcache")
	if err := os.MkdirAll(native, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := steamLocalHTMLCachePath(flatpak)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("steamLocalHTMLCachePath(%q) = %q, want %q", flatpak, got, want)
	}
}

// Older clients kept it directly under the root.
func TestHTMLCachePathAcceptsTheLegacyLayout(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "htmlcache")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := steamLocalHTMLCachePath(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %q, want the legacy %q", got, want)
	}
}

// With neither present the caller still needs a path to name in its
// "skipped, missing" line.
func TestHTMLCachePathNamesConfigWhenNeitherExists(t *testing.T) {
	root := t.TempDir()
	got, err := steamLocalHTMLCachePath(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "config", "htmlcache"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLCachePathRejectsAnUnknownRoot(t *testing.T) {
	if _, err := steamLocalHTMLCachePath("  "); err == nil {
		t.Error("expected an error when the install folder is not known")
	}
}
