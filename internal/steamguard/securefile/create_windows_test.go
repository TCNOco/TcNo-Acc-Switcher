//go:build windows

package securefile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateNewProtectsBeforeWriteAndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.maFile")
	file, err := CreateNew(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyProtected(path); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("sentinel")); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNew(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second create error = %v", err)
	}
}

func TestCreateDirectoryNewProtectsAndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected")
	if err := CreateDirectoryNew(path); err != nil {
		t.Fatal(err)
	}
	if err := verifyProtected(path); err != nil {
		t.Fatal(err)
	}
	if err := CreateDirectoryNew(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second create error = %v", err)
	}
}
