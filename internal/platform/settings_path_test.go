package platform

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/settingsfile"
)

func TestLoadSettings_migratesExeRootToPortableUserData(t *testing.T) {
	dir := t.TempDir()
	exeDir := filepath.Join(dir, "bin")
	portable := PortableUserDataDir(exeDir)
	if err := os.MkdirAll(portable, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(exeDir, settingsFileName)
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"version":1,"language":"en-US"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ResetPathSingletonsForTest(exeDir)
	if _, err := LoadAppSettings(exeDir); err != nil {
		t.Fatal(err)
	}

	migrated := filepath.Join(portable, settingsFileName)
	if _, err := os.Stat(migrated); err != nil {
		t.Fatalf("expected migrated settings at %s: %v", migrated, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy settings should be removed, err=%v", err)
	}
}

func TestSaveSettings_customUserDataUsesExeRoot(t *testing.T) {
	dir := t.TempDir()
	exeDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(dir, "custom", UserDataDirName)
	s := AppSettings{
		Version:          1,
		Language:         "en-US",
		UserDataPath:     custom,
		PlatformExePaths: map[string]string{},
	}
	ResetPathSingletonsForTest(exeDir)
	if err := SaveAppSettings(exeDir, s); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(exeDir, settingsfile.FileName)
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected settings at exe root %s: %v", want, err)
	}
}
