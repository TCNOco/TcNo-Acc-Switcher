package app

import (
	"log/slog"
	"math"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// gameModeCSSWidth is the width the UI is scaled to present, in CSS pixels.
//
// The layout needs 760px; 960 keeps a margin above that while making everything
// roughly twice the size on a 1080p handheld, where a 1:1 UI is unusable.
const gameModeCSSWidth = 960

// gameModeZoom scales the UI to the panel it is actually on: a flat 2x would
// leave a 1280-wide Steam Deck with 640 CSS pixels, below the width the layout
// needs. The cap keeps a large TV from magnifying the UI into uselessness.
func gameModeZoom(screenWidth int) float64 {
	if screenWidth <= 0 {
		return 1
	}
	return math.Max(1, math.Min(2, float64(screenWidth)/gameModeCSSWidth))
}

// Explicit scale applies to the main window. Automatic keeps handheld scaling.
func applyInterfaceScale(win application.Window, scale float64) {
	if win == nil {
		return
	}
	gameMode := platform.InGamescopeSession()
	width := 0
	if scale == 0 && gameMode {
		screen, err := win.GetScreen()
		if err == nil && screen != nil {
			width = screen.Size.Width
		} else {
			slog.Warn("game mode: could not read the screen, scaling to 2x", "err", err)
		}
	}
	zoom := mainWindowZoom(scale, gameMode, width)
	win.SetMinSize(int(math.Round(mainWindowMinWidth*zoom)), int(math.Round(mainWindowMinHeight*zoom)))
	win.SetZoom(zoom)
}

func mainWindowZoom(scale float64, gameMode bool, screenWidth int) float64 {
	if scale != 0 {
		return scale
	}
	if !gameMode {
		return 1
	}
	if screenWidth <= 0 {
		return 2
	}
	return gameModeZoom(screenWidth)
}
