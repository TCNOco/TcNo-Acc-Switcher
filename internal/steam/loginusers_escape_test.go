package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoginUsersRoundTripKeepsEscaping is the regression for a measured
// corruption: parse and write were not inverse, so a persona name holding one
// backslash gained one on every account switch. Measured before the fix, on
// this exact input: 2, 4, 8, then 16 backslashes across three switches.
//
// A switch rewrites this whole file, so anything in it that is not a plain
// word - a persona name with a backslash or a quote in it - depends on the
// round trip being lossless.
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

// TestParseLoginUsersResolvesEscapes checks the value the rest of the app sees:
// a name is handed over as the user typed it, not as the file spells it. The
// account list and the tray both show this string.
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
