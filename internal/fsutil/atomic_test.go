package fsutil

import (
	"errors"
	"testing"
)

func TestRenameWithRetry_NonRetriableFailsImmediately(t *testing.T) {
	attempts := 0
	err := renameWithRetry("old", "new", func(string, string) error {
		attempts++
		return errors.New("no such file")
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt for a non-retriable error, got %d", attempts)
	}
}
