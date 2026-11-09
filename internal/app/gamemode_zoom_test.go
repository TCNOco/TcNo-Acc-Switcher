package app

import "testing"

func TestMainWindowZoomManualAndAutomatic(t *testing.T) {
	for _, tc := range []struct {
		scale    float64
		gameMode bool
		width    int
		want     float64
	}{
		{0, false, 1920, 1}, {0, true, 1280, 1280.0 / gameModeCSSWidth},
		{0, true, 0, 2}, {0.75, false, 1920, 0.75}, {1.25, true, 1280, 1.25},
	} {
		if got := mainWindowZoom(tc.scale, tc.gameMode, tc.width); got != tc.want {
			t.Fatalf("manual=%v handheld=%t width=%d: got %v want %v", tc.scale, tc.gameMode, tc.width, got, tc.want)
		}
	}
}

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
