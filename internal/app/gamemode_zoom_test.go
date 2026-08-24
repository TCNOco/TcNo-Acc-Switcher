package app

import "testing"

func TestGameModeZoom(t *testing.T) {
	cases := []struct {
		name        string
		screenWidth int
		want        float64
	}{
		{name: "ROG Ally and most 1080p handhelds", screenWidth: 1920, want: 2},
		// A flat 2x here would leave 640 CSS pixels, under the 760 the layout needs.
		{name: "Steam Deck", screenWidth: 1280, want: 1280.0 / gameModeCSSWidth},
		{name: "4K television, capped", screenWidth: 3840, want: 2},
		{name: "small panel never shrinks the UI", screenWidth: 800, want: 1},
		{name: "unknown screen", screenWidth: 0, want: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gameModeZoom(tc.screenWidth); got != tc.want {
				t.Fatalf("gameModeZoom(%d) = %v, want %v", tc.screenWidth, got, tc.want)
			}
		})
	}
}

func TestGameModeZoomNeverGoesBelowOne(t *testing.T) {
	for width := -100; width < 2000; width += 37 {
		if got := gameModeZoom(width); got < 1 {
			t.Fatalf("gameModeZoom(%d) = %v, want at least 1", width, got)
		}
	}
}
