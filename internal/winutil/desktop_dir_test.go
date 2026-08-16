package winutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setFakeHome points os.UserHomeDir at a scratch directory for the current platform.
func setFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}
	return home
}

func TestDesktopWriteDirPrefersOneDriveWhenProfileDesktopIsGone(t *testing.T) {
	home := setFakeHome(t)
	oneDrive := filepath.Join(home, "OneDrive", "Desktop")
	if err := os.MkdirAll(oneDrive, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DesktopWriteDir()
	if err != nil {
		t.Fatalf("DesktopWriteDir() error = %v", err)
	}
	if got != oneDrive {
		t.Fatalf("DesktopWriteDir() = %q, want %q", got, oneDrive)
	}
}

func TestDesktopWriteDirPrefersProfileDesktopWhenBothExist(t *testing.T) {
	home := setFakeHome(t)
	plain := filepath.Join(home, "Desktop")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "OneDrive", "Desktop"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DesktopWriteDir()
	if err != nil {
		t.Fatalf("DesktopWriteDir() error = %v", err)
	}
	if got != plain {
		t.Fatalf("DesktopWriteDir() = %q, want %q", got, plain)
	}
}

func TestDesktopWriteDirFailsInsteadOfCreatingADesktop(t *testing.T) {
	home := setFakeHome(t)

	if got, err := DesktopWriteDir(); err == nil {
		t.Fatalf("DesktopWriteDir() = %q, want error when no Desktop exists", got)
	}
	if _, err := os.Stat(filepath.Join(home, "Desktop")); !os.IsNotExist(err) {
		t.Fatalf("DesktopWriteDir() created %s; it must never invent a Desktop folder", filepath.Join(home, "Desktop"))
	}
}
