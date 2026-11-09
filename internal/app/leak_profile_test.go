package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoroutineLeakReportIsLocalAndDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goroutine-leaks.txt")
	if err := writeGoroutineLeakProfile(path); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(body), "goroutineleak profile:") {
		t.Fatalf("missing leak profile: %q %v", body, err)
	}
	if err := writeGoroutineLeakProfile(path); !os.IsExist(err) {
		t.Fatalf("existing diagnostic report was overwritten: %v", err)
	}
	if err := writeGoroutineLeakProfile("relative.txt"); err == nil {
		t.Fatal("relative diagnostic output accepted")
	}
}
