//go:build windows

package vault

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// A protected directory whose rights stopped at itself handed every folder
// created inside an empty DACL, locking out the owner. Copying the vault, or
// letting any tool create a folder in it, produced a vault nobody could read.
func TestHardenedDirectoryPassesAccessToChildren(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := (platformHardener{}).HardenDir(root); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(root, "generation-copied-in")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "header.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadDir(child); err != nil {
		t.Fatalf("a folder created inside the vault cannot be read: %v", err)
	}
}

func TestHardenedDirectoryStaysProtected(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := (platformHardener{}).HardenDir(root); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("vault directory no longer refuses inherited rights")
	}
}

// Open repairs a generation folder left unreadable by an earlier copy: its path
// is known from the active file, so the repair does not depend on listing the
// vault, which is the thing that fails.
func TestOpenRepairsUnreadableGeneration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	v, err := Create(root, "generation repair password")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Lock(); err != nil {
		t.Fatal(err)
	}
	active, err := readActive(root)
	if err != nil {
		t.Fatal(err)
	}
	genPath, err := generationPath(root, active)
	if err != nil {
		t.Fatal(err)
	}

	denyAll(t, genPath)
	if _, err := os.ReadDir(genPath); err == nil {
		t.Skip("this account cannot be locked out of its own directory")
	}

	reopened, err := Open(root)
	if err != nil {
		t.Fatalf("open did not repair the generation folder: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Lock() })
	if _, err := os.ReadDir(genPath); err != nil {
		t.Fatalf("generation folder still unreadable after open: %v", err)
	}
}

// denyAll strips every right from a directory, reproducing what a copy into a
// protected folder used to leave behind. The owner keeps the right to put the
// rights back, which both Open and the cleanup rely on.
func denyAll(t *testing.T, path string) {
	t.Helper()
	descriptor, err := windows.SecurityDescriptorFromString("D:P")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Without this the temporary directory cannot be removed when the test ends.
	t.Cleanup(func() { _ = (platformHardener{}).HardenDir(path) })
}
