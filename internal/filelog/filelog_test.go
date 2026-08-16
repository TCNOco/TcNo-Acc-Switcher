package filelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterRotatesAtTheCapAndKeepsOneGeneration(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "app.log", 32)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	for i := 0; i < 6; i++ {
		if _, err := w.Write([]byte("0123456789\n")); err != nil {
			t.Fatal(err)
		}
	}

	cur, err := os.Stat(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if cur.Size() > 32 {
		t.Errorf("current log is %d bytes, past the 32 byte cap", cur.Size())
	}
	if _, err := os.Stat(filepath.Join(dir, "app.log.1")); err != nil {
		t.Fatalf("previous generation missing: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected exactly 2 files, got %d", len(entries))
	}
}

// A record must land in one file or the other, never be torn across a rotation.
func TestWriterDoesNotSplitARecordAcrossGenerations(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "app.log", 20)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	for _, line := range []string{"first-record\n", "second-record\n"} {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}

	cur, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(cur); got != "second-record\n" {
		t.Errorf("current log = %q, want the whole second record", got)
	}
	prev, err := os.ReadFile(filepath.Join(dir, "app.log.1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(prev); got != "first-record\n" {
		t.Errorf("rotated log = %q, want the whole first record", got)
	}
}

// A log must survive a restart, or the record of what happened before a crash
// is exactly what gets lost.
func TestOpenAppendsToAnExistingLog(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.log"), []byte("earlier\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := Open(dir, "app.log", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Write([]byte("later\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "earlier\n") {
		t.Errorf("existing log was truncated: %q", string(got))
	}
	if !strings.Contains(string(got), "later\n") {
		t.Errorf("new record missing: %q", string(got))
	}
}
