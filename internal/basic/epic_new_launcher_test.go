package basic

import (
	"path/filepath"
	"testing"
)

func TestParseEpicNewLauncherAccountID(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := parseEpicNewLauncherAccountID([]byte(`{"account_id":"abc123-456"}`))
		if err != nil {
			t.Fatalf("parseEpicNewLauncherAccountID returned error: %v", err)
		}
		if got != "abc123-456" {
			t.Fatalf("account ID = %q, want %q", got, "abc123-456")
		}
	})

	for _, tc := range []struct {
		name string
		data string
	}{
		{name: "invalid JSON", data: "{"},
		{name: "missing ID", data: `{"other":"id"}`},
		{name: "empty ID", data: `{"account_id":"  "}`},
		{name: "whitespace", data: `{"account_id":"abc 123"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseEpicNewLauncherAccountID([]byte(tc.data)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestValidateEpicNewLauncherAuthFiles(t *testing.T) {
	valid := map[string][]byte{
		epicNewLauncherAccessTokenFile: {1, 2, 3},
		epicNewLauncherAuthInfoFile:    []byte(`{"prod":{}}`),
	}
	if err := validateEpicNewLauncherAuthFiles(valid); err != nil {
		t.Fatalf("valid auth snapshot rejected: %v", err)
	}

	cases := []struct {
		name  string
		files map[string][]byte
	}{
		{name: "missing token", files: map[string][]byte{epicNewLauncherAuthInfoFile: []byte("{}")}},
		{name: "invalid auth JSON", files: map[string][]byte{epicNewLauncherAccessTokenFile: {1}, epicNewLauncherAuthInfoFile: []byte("{")}},
		{name: "path traversal", files: map[string][]byte{epicNewLauncherAccessTokenFile: {1}, epicNewLauncherAuthInfoFile: []byte("{}"), "../outside": []byte("bad")}},
		{name: "oversized file", files: map[string][]byte{epicNewLauncherAccessTokenFile: {1}, epicNewLauncherAuthInfoFile: []byte("{}"), "large.bin": make([]byte, epicNewLauncherMaxAuthFileSize+1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateEpicNewLauncherAuthFiles(tc.files); err == nil {
				t.Fatal("expected invalid auth snapshot to be rejected")
			}
		})
	}
}

func TestIsEpicNewLauncherFolder(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	root := filepath.Join(local, "Epic Games", "Launcher")

	if !isEpicNewLauncherFolder(root) {
		t.Fatal("new launcher install folder was not recognized")
	}
	if !isEpicNewLauncherFolder(filepath.Join(root, "EpicGamesLauncher.exe")) {
		t.Fatal("new launcher executable path was not recognized")
	}
	if isEpicNewLauncherFolder(filepath.Join(root, "Portal", "Binaries", "Win32")) {
		t.Fatal("nested legacy launcher path was incorrectly recognized as the new launcher")
	}
	if isEpicNewLauncherFolder(filepath.Join(local, "EpicGamesLauncher")) {
		t.Fatal("legacy launcher folder was incorrectly recognized as the new launcher")
	}
}
