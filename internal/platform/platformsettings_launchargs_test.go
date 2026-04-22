package platform

import "testing"

func TestHasLaunchArgToken(t *testing.T) {
	if !HasLaunchArgToken("-silent -dev", "-silent") {
		t.Fatal("expected silent")
	}
	if !HasLaunchArgToken("-SILENT", "-silent") {
		t.Fatal("case insensitive")
	}
	if HasLaunchArgToken("-dev", "-silent") {
		t.Fatal("no silent")
	}
}

func TestEnsureRemoveLaunchArg(t *testing.T) {
	line := EnsureLaunchArg("-dev", "-silent")
	if line != "-dev -silent" {
		t.Fatalf("got %q", line)
	}
	line2 := EnsureLaunchArg(line, "-silent")
	if line2 != "-dev -silent" {
		t.Fatalf("duplicate: got %q", line2)
	}
	rm := RemoveLaunchArgToken("-dev -silent -vgui", "-silent")
	if rm != "-dev -vgui" {
		t.Fatalf("remove: got %q", rm)
	}
}
