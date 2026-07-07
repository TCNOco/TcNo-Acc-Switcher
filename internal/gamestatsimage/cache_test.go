package gamestatsimage

import "testing"

func TestFilenameFromURL(t *testing.T) {
	t.Parallel()
	got, err := FilenameFromURL("https://api.mozambiquehe.re/assets/ranks/platinum4.png")
	if err != nil {
		t.Fatal(err)
	}
	if got != "platinum4.png" {
		t.Fatalf("got %q", got)
	}
}
