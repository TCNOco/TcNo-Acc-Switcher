//go:build windows

package fsutil

import (
	"errors"
	"os"
	"testing"
)

func TestRenameWithRetry_RetriesSharingViolation(t *testing.T) {
	attempts := 0
	err := renameWithRetry("old", "new", func(string, string) error {
		attempts++
		if attempts < 3 {
			return &os.LinkError{Op: "rename", Old: "old", New: "new", Err: errnoSharingViolation}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil after retries, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRenameWithRetry_GivesUpAfterBoundedAttempts(t *testing.T) {
	attempts := 0
	stuck := &os.LinkError{Op: "rename", Old: "old", New: "new", Err: errnoAccessDenied}
	err := renameWithRetry("old", "new", func(string, string) error {
		attempts++
		return stuck
	})
	if !errors.Is(err, errnoAccessDenied) {
		t.Fatalf("expected the last error to propagate, got %v", err)
	}
	if attempts != renameAttempts {
		t.Fatalf("expected %d attempts, got %d", renameAttempts, attempts)
	}
}
