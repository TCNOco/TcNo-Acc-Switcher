package app

import (
	"log/slog"
	"math"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// gameModeCSSWidth is the width the UI is scaled to present, in CSS pixels.
//
// The window declares a 760px minimum, which is what the layout needs before it
// starts breaking. 960 keeps a margin above that while making everything roughly
// twice the size on a 1080p handheld - a 7" screen at arm's length is nothing
// like a monitor, and at 1:1 the UI is unusable there.
const gameModeCSSWidth = 960

// gameModeZoom scales the UI to the panel it is actually on.
//
// A flat 2x would be right for the 1920-wide handhelds and wrong for a
// 1280-wide Steam Deck, where it would leave 640 CSS pixels - below the width
// the layout needs. Deriving it from the screen gives 2x on the first and a
// sane 1.33x on the second, and the cap keeps a large TV from magnifying the UI
// into uselessness.
func gameModeZoom(screenWidth int) float64 {
	if screenWidth <= 0 {
		return 1
	}
	return math.Max(1, math.Min(2, float64(screenWidth)/gameModeCSSWidth))
}

// applyGameModeZoom magnifies the UI when there is one app on a handheld screen.
// Anywhere else the user's own window size is the answer and this does nothing.
func applyGameModeZoom(win application.Window) {
	if win == nil || !platform.InGamescopeSession() {
		return
	}
	screen, err := win.GetScreen()
	if err != nil || screen == nil {
		// Better a readable guess than a UI too small to operate.
		slog.Warn("game mode: could not read the screen, scaling to 2x", "err", err)
		win.SetZoom(2)
		return
	}
	zoom := gameModeZoom(screen.Size.Width)
	slog.Info("game mode: scaling the UI to the panel", "screenWidth", screen.Size.Width, "zoom", zoom)
	win.SetZoom(zoom)
}
