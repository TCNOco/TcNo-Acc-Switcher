package paths

import "testing"

func TestShellShortcutBaseName_TrimsOuterUnderscores(t *testing.T) {
	// Decorative symbols become underscores then collapse; leading/trailing _ should strip.
	got := ShellShortcutBaseName("♡❤︎ Cinna ♡♥", 180)
	if got != "Cinna" {
		t.Fatalf("got %q want Cinna", got)
	}
}

func TestShellShortcutBaseName_KeepsInnerUnderscores(t *testing.T) {
	got := ShellShortcutBaseName("foo_bar baz", 180)
	if got != "foo_bar baz" {
		t.Fatalf("got %q want foo_bar baz", got)
	}
}
