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

	orig := os.Getenv("APPDATA")
	if err := os.Setenv("APPDATA", filepath.Join(dir, "appdata")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Setenv("APPDATA", orig) })

	got, ok := Discover(exeDir)
	if !ok {
		t.Fatal("expected settings file")
	}
	if got != filepath.Join(portable, FileName) {
		t.Fatalf("got %q, want portable settings", got)
	}
}

func TestDiscover_fallsBackToExeRoot(t *testing.T) {
	orig := os.Getenv("APPDATA")
	tmpAppData := filepath.Join(t.TempDir(), "appdata")
	if err := os.MkdirAll(tmpAppData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APPDATA", tmpAppData); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Setenv("APPDATA", orig) })

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
