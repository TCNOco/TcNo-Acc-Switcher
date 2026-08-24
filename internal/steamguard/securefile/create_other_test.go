//go:build !windows

package securefile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A Steam Guard secret must never be readable by another account, not even for
// the instant between creating the file and tightening it.
func TestCreateNewIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	file, err := CreateNew(path)
	if err != nil {
		t.Fatalf("CreateNew: %v", err)
	}
	defer file.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("mode = %#o, want no group or other bits", perm)
	}
}

// Reusing a file somebody else created would inherit their permissions.
func TestCreateNewRefusesAnExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := CreateNew(path)
	if err == nil {
		file.Close()
		t.Fatal("CreateNew overwrote an existing file")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Errorf("err = %v, want ErrExist", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "theirs" {
		t.Errorf("existing file was modified: %q", got)
	}
}

// Following a planted symlink would put the secret wherever it pointed.
func TestCreateNewRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere")
	link := filepath.Join(dir, "secret")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	file, err := CreateNew(link)
	if err == nil {
		file.Close()
		t.Fatal("CreateNew followed a symlink")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("symlink target was created: %v", err)
	}
}

func TestCreateDirectoryNewIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault")
	if err := CreateDirectoryNew(path); err != nil {
		t.Fatalf("CreateDirectoryNew: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		t.Errorf("mode = %#o, want no group or other bits", perm)
	}
	if perm&0o100 == 0 {
		t.Errorf("mode = %#o, owner cannot enter the directory", perm)
	}
}
