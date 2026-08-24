package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// setTestAppData sets %APPDATA% to a temporary directory for the duration of the test
// and restores the original value on cleanup. This prevents tests from seeing the
// host machine's real TcNo Account Switcher config in %AppData%.
// setTestAppData points the user data directory at a temp dir.
//
// All three variables, because DefaultUserDataDir goes through
// os.UserConfigDir: that reads APPDATA on Windows but XDG_CONFIG_HOME, then
// HOME, everywhere else. With only APPDATA set, these tests read and write the
// real user's config directory off Windows - which made them depend on what the
// previous test happened to leave there.
func setTestAppData(t *testing.T) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "appdata")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("create temp appdata: %v", err)
	}
	for _, key := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME"} {
		orig := os.Getenv(key)
		if err := os.Setenv(key, tmp); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
		t.Cleanup(func() { _ = os.Setenv(key, orig) })
	}
}
