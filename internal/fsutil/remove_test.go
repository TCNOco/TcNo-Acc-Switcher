package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoveAllWithRetry_MissingPathReturnsNil(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope")
	if err := RemoveAllWithRetry(missing, 50*time.Millisecond, os.RemoveAll); err != nil {
		t.Fatalf("expected nil for missing path, got %v", err)
	}
}

func TestRemoveAllWithRetry_DeletesExistingPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "victim")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := RemoveAllWithRetry(target, 500*time.Millisecond, os.RemoveAll); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected path to be removed, stat err = %v", err)
	}
}

func TestRemoveAllWithRetry_RetriesUntilSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "flaky")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	attempts := 0
	gate := func(path string) error {
		attempts++
		if attempts < 3 {
			return errors.New("synthetic lock")
		}
		return os.RemoveAll(path)
	}
	err := RemoveAllWithRetry(target, 2*time.Second, gate)
	if err != nil {
		t.Fatalf("expected nil after retries, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected target to be removed, stat err = %v", err)
	}
}

func TestRemoveAllWithRetry_FailsAfterDeadline(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "stuck")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	always := func(string) error { return errors.New("perma-locked") }
	start := time.Now()
	err := RemoveAllWithRetry(target, 250*time.Millisecond, always)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected error after deadline")
	}
	if !strings.Contains(err.Error(), "perma-locked") {
		t.Fatalf("expected last error to propagate, got %v", err)
	}
	if elapsed < 200*time.Millisecond {
		t.Fatalf("expected loop to honour deadline, ran for %v", elapsed)
	}
}
