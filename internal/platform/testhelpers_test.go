package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// setTestAppData points the config dir at a temporary directory so tests never see
// the host machine's real TcNo Account Switcher config.
// All three variables, because DefaultUserDataDir goes through os.UserConfigDir:
// that reads APPDATA on Windows, but XDG_CONFIG_HOME then HOME everywhere else.
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
