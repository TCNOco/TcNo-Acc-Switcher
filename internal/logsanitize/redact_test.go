package logsanitize

import (
	"sort"
	"strings"
	"testing"
)

func TestAliasForAccount_trailingSpecials(t *testing.T) {
	got := aliasForAccount("account1", "kev_in!@#")
	if got != "account1!@#" {
		t.Fatalf("alias = %q, want account1!@#", got)
	}
}

func TestAliasForAccount_alphanumericOnly(t *testing.T) {
	got := aliasForAccount("account2", "76561198123456789")
	if got != "account2" {
		t.Fatalf("alias = %q, want account2", got)
	}
}

func TestReplaceCI_overlapping(t *testing.T) {
	s := replaceCI("ABCDEF", "abc", "X")
	if s != "XDEF" {
		t.Fatalf("got %q", s)
	}
}

func TestRedact_noAccounts(t *testing.T) {
	in := "path C:\\Users\\kevin\\file.txt"
	if got := Redact(in); got != in {
		t.Fatalf("Redact without accounts changed text: %q", got)
	}
}

func TestCollectReplacements_sortLongestFirst(t *testing.T) {
	reps := []secretReplacement{
		{secret: "ab", replacement: "account1"},
		{secret: "abcd", replacement: "account2"},
	}
	sort.Slice(reps, func(i, j int) bool {
		return len(reps[i].secret) > len(reps[j].secret)
	})
	if reps[0].secret != "abcd" {
		t.Fatal("expected longest secret first")
	}
}

func TestReplaceCI_preservesSurrounding(t *testing.T) {
	got := replaceCI(`failed copy C:\cache\Kevin\data`, "kevin", "account1")
	if !strings.Contains(strings.ToLower(got), "account1") {
		t.Fatalf("got %q", got)
	}
}
