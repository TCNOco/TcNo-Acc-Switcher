package cli

import (
	"path/filepath"
	"testing"
)

func TestGoroutineLeakProfileFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goroutine-leaks.txt")
	flag := "--goroutine-leak-profile=" + path
	parsed, err := Parse([]string{flag}, nil)
	if err != nil || parsed.GoroutineLeakProfile != path || parsed.Kind != KindNone {
		t.Fatalf("diagnostic flag parsed as %#v: %v", parsed, err)
	}
	for _, args := range [][]string{{"--goroutine-leak-profile="}, {"--goroutine-leak-profile=relative.txt"}, {flag, flag}} {
		if _, err := Parse(args, nil); err == nil {
			t.Fatalf("invalid diagnostic args accepted: %v", args)
		}
	}
}
