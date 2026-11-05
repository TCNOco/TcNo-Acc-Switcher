package authenticatorstorage

import (
	"os"
	"path/filepath"
	"testing"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(os.Getenv("GOTMPDIR"), "authenticator-storage-")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func expectFile(t *testing.T, path, text string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != text {
		t.Fatalf("file %s: %q, %v", path, raw, err)
	}
}

func TestLegacyDirectoriesCopyWithoutChangingOriginals(t *testing.T) {
	for _, mapping := range [][2]string{{"SteamGuard", "Authenticators"}, {"SteamBrowser", "AccountBrowser"}} {
		root := fixtureRoot(t)
		source := filepath.Join(root, mapping[0], "records", "fixture.bin")
		writeFixture(t, source, "fixture ciphertext")
		target, err := Directory(root, mapping[0], mapping[1])
		if err != nil {
			t.Fatal(err)
		}
		if target != filepath.Join(root, mapping[1]) {
			t.Fatal("wrong generic destination")
		}
		expectFile(t, filepath.Join(target, "records", "fixture.bin"), "fixture ciphertext")
		expectFile(t, source, "fixture ciphertext")
	}
}

func TestInterruptedDirectoryCopyResumes(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, filepath.Join(root, "SteamGuard", "record.bin"), "complete ciphertext")
	writeFixture(t, filepath.Join(root, "Authenticators", started), "copying\n")
	writeFixture(t, filepath.Join(root, "Authenticators", "record.bin"), "partial")
	target, err := Directory(root, "SteamGuard", "Authenticators")
	if err != nil {
		t.Fatal(err)
	}
	expectFile(t, filepath.Join(target, "record.bin"), "complete ciphertext")
	expectFile(t, filepath.Join(target, completed), "complete\n")
}

func TestExistingGenericStoreWinsWithoutMerging(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, filepath.Join(root, "SteamGuard", "record.bin"), "legacy")
	writeFixture(t, filepath.Join(root, "Authenticators", "record.bin"), "current")
	target, err := Directory(root, "SteamGuard", "Authenticators")
	if err != nil {
		t.Fatal(err)
	}
	expectFile(t, filepath.Join(target, "record.bin"), "current")
	expectFile(t, filepath.Join(root, "SteamGuard", "record.bin"), "legacy")
}

func TestSettingsMigrationPreservesPreferencesAndOriginal(t *testing.T) {
	root := fixtureRoot(t)
	old := filepath.Join(root, "Settings", "SteamGuard.json")
	writeFixture(t, old, `{"version":1,"featureEnabled":false,"rememberPasswordForSession":true}`)
	target, err := Settings(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(target) != "Authenticators.json" {
		t.Fatal("wrong settings filename")
	}
	expectFile(t, target, `{"version":1,"featureEnabled":false,"rememberPasswordForSession":true}`)
	expectFile(t, old, `{"version":1,"featureEnabled":false,"rememberPasswordForSession":true}`)
}

func TestCompletedMigrationDoesNotResurrectLegacyStore(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, filepath.Join(root, "SteamGuard", "record.bin"), "legacy")
	writeFixture(t, filepath.Join(root, ".authenticator-storage-Authenticators"), "complete\n")
	target, err := Directory(root, "SteamGuard", "Authenticators")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("previously migrated legacy store was imported again")
	}
	expectFile(t, filepath.Join(root, "SteamGuard", "record.bin"), "legacy")
}

func TestCompletedSettingsMigrationDoesNotRestoreOldPreferences(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, filepath.Join(root, "Settings", "SteamGuard.json"), `{"version":1}`)
	writeFixture(t, filepath.Join(root, "Settings", ".authenticator-settings-migration-complete"), "complete\n")
	target, err := Settings(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("old settings were imported after a completed migration")
	}
}
