package shortcutsvdf

import "testing"

func TestShortcutAppIDMatchesTheCommunityVector(t *testing.T) {
	// The vector every community implementation agrees on, with the quote bytes
	// inside the hashed string.
	got := ShortcutAppID(`"C:\Program Files (x86)\XBMC\XBMC.exe"`, "XBMC")
	if want := int32(-2119177653); got != want {
		t.Fatalf("ShortcutAppID = %d, want %d", got, want)
	}
}

func TestQuotesAreInsideTheHash(t *testing.T) {
	// Hashing the unquoted path yields a different id, which orphans the artwork
	// Steam files under the old one.
	quoted := ShortcutAppID(`"/usr/bin/app"`, "App")
	bare := ShortcutAppID("/usr/bin/app", "App")
	if quoted == bare {
		t.Fatal("quoted and unquoted Exe hashed alike")
	}
}

func TestShortcutAppIDIsAlwaysNegative(t *testing.T) {
	// The high bit is forced, so the signed field Steam stores never goes positive.
	for _, name := range []string{"", "a", "TcNo Account Switcher", "zzzzzzzzzzzz"} {
		if got := ShortcutAppID(`"/x"`, name); got >= 0 {
			t.Fatalf("ShortcutAppID(%q) = %d, want a negative id", name, got)
		}
	}
}
