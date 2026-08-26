package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A switch rewrites this whole file, so parse and write have to be inverse:
// otherwise every backslash in a persona name doubles on every switch.
func TestLoginUsersRoundTripKeepsEscaping(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := LoginUsersPath(dir)

	// One literal backslash and one literal quote, escaped the way Steam writes
	// them. Both are legal in a Steam persona name.
	const personaOnDisk = `back\\slash and \"quotes\"`
	initial := "\"users\"\n{\n\t\"76561198000000100\"\n\t{\n\t\t\"AccountName\"\t\t\"alpha\"\n\t\t\"PersonaName\"\t\t\"" +
		personaOnDisk + "\"\n\t\t\"MostRecent\"\t\t\"1\"\n\t}\n}\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 3; i++ {
		users, err := ParseLoginUsers(path)
		if err != nil {
			t.Fatalf("parse before switch %d: %v", i, err)
		}
		if err := os.WriteFile(path, KeyValueToText(LoginUsersToKeyValue(users)), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), personaOnDisk) {
			t.Fatalf("switch %d changed the escaping of the persona name:\n%s", i, got)
		}
	}
}

// The account list and tray show this string, so it must be the name as typed,
// not as the file spells it.
func TestParseLoginUsersResolvesEscapes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := LoginUsersPath(dir)
	contents := "\"users\"\n{\n\t\"76561198000000100\"\n\t{\n\t\t\"AccountName\"\t\t\"alpha\"\n\t\t\"PersonaName\"\t\t\"back\\\\slash\"\n\t\t\"MostRecent\"\t\t\"1\"\n\t}\n}\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	users, err := ParseLoginUsers(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("users = %d, want 1", len(users))
	}
	if got, want := users[0].PersonaName, `back\slash`; got != want {
		t.Errorf("PersonaName = %q, want %q", got, want)
	}
}
