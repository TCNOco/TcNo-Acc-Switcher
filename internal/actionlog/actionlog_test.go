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

func TestRecord_redactsLabelledSecretsButKeepsAccountIdentifiers(t *testing.T) {
	Init()
	Record(
		"steamguard:save",
		"accountID=76561198123456789 username=ordinary_username",
		`{"access_token":"ACTION_TOKEN_SENTINEL","account_name":"ordinary_username"}`,
		errors.New("wrapped: identity_secret=ACTION_ERROR_SENTINEL"),
	)
	got := SnapshotPruned(100, 300)
	for _, secret := range []string{"ACTION_TOKEN_SENTINEL", "ACTION_ERROR_SENTINEL"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
	for _, public := range []string{"76561198123456789", "ordinary_username"} {
		if !strings.Contains(got, public) {
			t.Fatalf("public identifier %q was removed from %q", public, got)
		}
	}
}
