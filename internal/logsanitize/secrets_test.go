package logsanitize

import (
	"os"
	"path/filepath"
	"testing"
)

// A loginusers.vdf caught mid-write: the key is there, its value is not.
// steamvdf panics on this rather than returning an error, and logsanitize runs
// on the logging path - a crash here would take down the app.
const truncatedLoginUsers = "\"users\"\n{\n\t\"76561198000000100\"\n\t{\n\t\t\"AccountName\""

func TestParseLoginUsers_truncatedFileErrorsNotPanics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loginusers.vdf")
	if err := os.WriteFile(path, []byte(truncatedLoginUsers), 0o600); err != nil {
		t.Fatal(err)
	}

	users, err := parseLoginUsers(path)
	if err == nil {
		t.Fatalf("parseLoginUsers(truncated) = %v, nil; want an error", users)
	}
	if users != nil {
		t.Errorf("users = %v, want nil on failure", users)
	}
}

func TestParseLoginUsers_wellFormedFileParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loginusers.vdf")
	content := truncatedLoginUsers + "\t\t\"kevin\"\n\t\t\"PersonaName\"\t\t\"Kev\"\n\t}\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	users, err := parseLoginUsers(path)
	if err != nil {
		t.Fatalf("parseLoginUsers error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1: %+v", len(users), users)
	}
	got := users[0]
	if got.steamID != "76561198000000100" || got.accountName != "kevin" || got.personaName != "Kev" {
		t.Errorf("user = %+v, want the written account", got)
	}
}
