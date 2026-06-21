package actionlog

import (
	"errors"
	"strings"
	"testing"
)

func TestSnapshotPruned_empty(t *testing.T) {
	Init()
	if got := SnapshotPruned(100, 300); got != "" {
		t.Fatalf("empty snapshot = %q, want empty", got)
	}
}

func TestSnapshotPruned_underLimit(t *testing.T) {
	Init()
	for i := 0; i < 50; i++ {
		Record("file:write", "path", "", nil)
	}
	got := SnapshotPruned(100, 300)
	if strings.Count(got, "\n") != 49 {
		t.Fatalf("expected 50 lines, got %d lines", strings.Count(got, "\n")+1)
	}
	if strings.Contains(got, "omitted") {
		t.Fatal("should not omit when under limit")
	}
}

func TestSnapshotPruned_overLimit(t *testing.T) {
	Init()
	for i := 0; i < 500; i++ {
		Record("file:write", "path", "", nil)
	}
	got := SnapshotPruned(100, 300)
	if !strings.Contains(got, "100 lines omitted") {
		t.Fatalf("expected omission marker, got:\n%s", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 100+1+300 {
		t.Fatalf("line count = %d, want %d", len(lines), 100+1+300)
	}
}

func TestRecord_failOutcome(t *testing.T) {
	Init()
	Record("registry:write", "HKCU\\Foo:Bar", "secret", errors.New("access denied"))
	got := SnapshotPruned(100, 300)
	if !strings.Contains(got, `outcome=fail`) || !strings.Contains(got, "access denied") {
		t.Fatalf("unexpected line: %q", got)
	}
}
