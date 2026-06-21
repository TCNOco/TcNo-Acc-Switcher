package updatecheck

import "testing"

func TestParsePlatformsJSONVersion(t *testing.T) {
	t.Parallel()
	v, err := ParsePlatformsJSONVersion([]byte(`{"Platforms":{},"Version":"4.0.1"}`))
	if err != nil || v != "4.0.1" {
		t.Fatalf("got %q err=%v", v, err)
	}
}

func TestIsVersionNewer(t *testing.T) {
	t.Parallel()
	if !IsVersionNewer("4.0.1", "4.0.0") {
		t.Fatal("expected 4.0.1 newer than 4.0.0")
	}
	if IsVersionNewer("4.0.0", "4.0.0") {
		t.Fatal("expected equal versions not newer")
	}
	if IsVersionNewer("4.0.0", "4.0.1") {
		t.Fatal("expected older remote rejected")
	}
	if !IsVersionNewer("4.0.0", "") {
		t.Fatal("expected empty current to accept remote")
	}
}
