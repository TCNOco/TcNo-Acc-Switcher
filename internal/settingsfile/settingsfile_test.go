package settingsfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_prefersPortableOverAppData(t *testing.T) {
	dir := t.TempDir()
	exeDir := filepath.Join(dir, "bin")
	portable := PortableUserDataDir(exeDir)
	appData := filepath.Join(dir, "appdata", UserDataDirName)
	for _, d := range []string{portable, appData} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(portable, FileName), []byte(`{"language":"portable"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appData, FileName), []byte(`{"language":"appdata"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	setTestUserConfig(t, filepath.Join(dir, "appdata"))

	got, ok := Discover(exeDir)
	if !ok {
		t.Fatal("expected settings file")
	}
	if got != filepath.Join(portable, FileName) {
		t.Fatalf("got %q, want portable settings", got)
	}
}

func TestDiscover_fallsBackToExeRoot(t *testing.T) {
	setTestUserConfig(t, filepath.Join(t.TempDir(), "appdata"))

	exeDir := t.TempDir()
	legacy := filepath.Join(exeDir, FileName)
	if err := os.WriteFile(legacy, []byte(`{"language":"legacy"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := Discover(exeDir)
	if !ok || got != legacy {
		t.Fatalf("got %q ok=%v, want %q", got, ok, legacy)
	}
}

func TestIsDefaultUserDataDir(t *testing.T) {
	exeDir := filepath.Join(t.TempDir(), "bin")
	portable := PortableUserDataDir(exeDir)
	custom := filepath.Join(t.TempDir(), "custom", UserDataDirName)
	if !IsDefaultUserDataDir(portable, exeDir) {
		t.Fatal("portable should be default")
	}
	if IsDefaultUserDataDir(custom, exeDir) {
		t.Fatal("custom should not be default")
	}
}

// setTestUserConfig points the user config directory at dir.
//
// All three variables, because DefaultSearchDirs goes through os.UserConfigDir:
// that reads APPDATA on Windows but XDG_CONFIG_HOME, then HOME, elsewhere. With
// only APPDATA set these tests find the real user's settings file, so they pass
// only on a machine where the app is not installed.
func setTestUserConfig(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME"} {
		orig := os.Getenv(key)
		if err := os.Setenv(key, dir); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
		t.Cleanup(func() { _ = os.Setenv(key, orig) })
	}
}
