package crashlog

import (
	"os"
	"testing"
)

func TestHasPendingAndDiscardPending(t *testing.T) {
	dir := t.TempDir()
	orig := crashDumpDirResolver
	crashDumpDirResolver = func() (string, error) { return dir, nil }
	t.Cleanup(func() { crashDumpDirResolver = orig })

	if HasPending() {
		t.Fatal("expected no pending crash dump")
	}

	dumpPath, err := crashDumpPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dumpPath, []byte(`{"error":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasPending() {
		t.Fatal("expected pending crash dump")
	}
	if err := DiscardPending(); err != nil {
		t.Fatal(err)
	}
	if HasPending() {
		t.Fatal("expected crash dump to be discarded")
	}
}
