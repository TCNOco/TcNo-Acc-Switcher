package basic

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

// ---------------------------------------------------------------------------
// parseRockstarEmail
// ---------------------------------------------------------------------------

func TestParseRockstarEmail(t *testing.T) {
	t.Parallel()
	if got := parseRockstarEmail([]byte(`<Profile><Email>user@rockstar.com</Email></Profile>`)); got != "user@rockstar.com" {
		t.Errorf("got %q, want user@rockstar.com", got)
	}
	if got := parseRockstarEmail([]byte(`<Email>test@example.org</Email>`)); got != "test@example.org" {
		t.Errorf("got %q", got)
	}
	if got := parseRockstarEmail([]byte(`no email here`)); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got := parseRockstarEmail([]byte(`<Email></Email>`)); got != "" {
		t.Errorf("empty Email: got %q", got)
	}
	// Whitespace padding
	if got := parseRockstarEmail([]byte(`<Email>  spaced@mail.com  </Email>`)); got != "spaced@mail.com" {
		t.Errorf("got %q, want spaced@mail.com", got)
	}
	// Multiple emails — returns first
	if got := parseRockstarEmail([]byte(`<Email>first@a.com</Email><Email>second@b.com</Email>`)); got != "first@a.com" {
		t.Errorf("got %q, want first@a.com", got)
	}
	// Email with encoded characters
	if got := parseRockstarEmail([]byte(`<Email>user+tag@domain.co.uk</Email>`)); got != "user+tag@domain.co.uk" {
		t.Errorf("got %q", got)
	}
}

// ---------------------------------------------------------------------------
// builtInUniqueIDRockstarEmail — end-to-end with temp files
// ---------------------------------------------------------------------------

func TestBuiltInUniqueIDRockstarEmail(t *testing.T) {
	dir := t.TempDir()

	// Create two profile files, newest should be picked
	older := filepath.Join(dir, "Profiles", "old_profile.xml")
	newer := filepath.Join(dir, "Profiles", "new_profile.xml")
	os.MkdirAll(filepath.Dir(older), 0o755)
	os.WriteFile(older, []byte(`<Profile><Email>old@rockstar.com</Email></Profile>`), 0o644)
	os.WriteFile(newer, []byte(`<Profile><Email>new@rockstar.com</Email></Profile>`), 0o644)

	id, err := builtInUniqueIDRockstarEmail(platform.Descriptor{
		UniqueIdFile: filepath.Join(dir, "Profiles", "*.xml"),
	}, platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("builtInUniqueIDRockstarEmail: %v", err)
	}
	if id != "new@rockstar.com" {
		t.Errorf("got %q, want new@rockstar.com (newest file)", id)
	}
}

func TestBuiltInUniqueIDRockstarEmail_NoFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := builtInUniqueIDRockstarEmail(platform.Descriptor{
		UniqueIdFile: filepath.Join(dir, "nope", "*.xml"),
	}, platform.PathTokenContext{})
	if err == nil {
		t.Fatal("expected error for no matching files")
	}
}

func TestBuiltInUniqueIDRockstarEmail_NoEmail(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Profiles", "data.xml")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(`<Profile><Info>no email tag</Info></Profile>`), 0o644)

	_, err := builtInUniqueIDRockstarEmail(platform.Descriptor{
		UniqueIdFile: filepath.Join(dir, "Profiles", "*.xml"),
	}, platform.PathTokenContext{})
	if err == nil {
		t.Fatal("expected error when no <Email> found")
	}
}
