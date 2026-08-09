package steamguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

func useSettingsRoot(t *testing.T) string {
	t.Helper()
	root := tempDir(t)
	paths.ResetForTest(root)
	return root
}

func TestSettingsDefaultAndRoundTrip(t *testing.T) {
	root := useSettingsRoot(t)
	got, err := LoadSettings()
	if err != nil || got.FeatureEnabled || got.RememberPasswordForSession {
		t.Fatalf("defaults = %#v, err = %v", got, err)
	}
	want := Settings{
		FeatureEnabled:             true,
		RememberPasswordForSession: true,
		LastVerifiedBackup:         time.Unix(1_700_000_000, 0).UTC().Format(time.RFC3339),
		LastVerifiedBackupPath:     filepath.Join(root, "Backups", "steam-guard.tsgbackup"),
	}
	if err := SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err = LoadSettings()
	if err != nil || !got.FeatureEnabled || !got.RememberPasswordForSession || got.LastVerifiedBackup != want.LastVerifiedBackup || got.LastVerifiedBackupPath != want.LastVerifiedBackupPath {
		t.Fatalf("round trip = %#v, err = %v", got, err)
	}
	path := filepath.Join(root, "Settings", settingsName)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsRejectsUnknownAndBadTimestamp(t *testing.T) {
	root := useSettingsRoot(t)
	dir := filepath.Join(root, "Settings")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, settingsName)
	for _, data := range []string{
		`{"version":1,"featureEnabled":false,"rememberPasswordForSession":false,"extra":true}`,
		`{"version":2,"featureEnabled":false,"rememberPasswordForSession":false}`,
		`{"version":1,"featureEnabled":false,"rememberPasswordForSession":false,"lastVerifiedBackup":"yesterday"}`,
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSettings(); !errors.Is(err, ErrInvalidSettings) {
			t.Fatalf("Load(%s) error = %v", data, err)
		}
	}
}
