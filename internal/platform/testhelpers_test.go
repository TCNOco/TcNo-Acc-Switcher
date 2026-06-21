package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// setTestAppData sets %APPDATA% to a temporary directory for the duration of the test
// and restores the original value on cleanup. This prevents tests from seeing the
// host machine's real TcNo Account Switcher config in %AppData%.
func setTestAppData(t *testing.T) {
	t.Helper()
	orig := os.Getenv("APPDATA")
	tmp := filepath.Join(t.TempDir(), "appdata")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("create temp appdata: %v", err)
	}
	if err := os.Setenv("APPDATA", tmp); err != nil {
		t.Fatalf("set APPDATA: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("APPDATA", orig) })
}
