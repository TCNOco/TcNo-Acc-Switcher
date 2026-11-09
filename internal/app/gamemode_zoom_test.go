package app

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type scaleStartupWindow struct {
	application.Window
	ready                   func(*application.WindowEvent)
	minWidth, minHeight     int
	zoom                    float64
	minSizeCalls, zoomCalls int
}

func (w *scaleStartupWindow) OnWindowEvent(event events.WindowEventType, handler func(*application.WindowEvent)) func() {
	if event == events.Common.WindowRuntimeReady {
		w.ready = handler
	}
	return func() {}
}

func (w *scaleStartupWindow) SetMinSize(width, height int) application.Window {
	w.minWidth, w.minHeight = width, height
	w.minSizeCalls++
	return w
}

func (w *scaleStartupWindow) SetZoom(zoom float64) application.Window {
	w.zoom = zoom
	w.zoomCalls++
	return w
}

func TestInitialInterfaceScaleWaitsForRuntime(t *testing.T) {
	for _, scale := range []float64{0.75, 1, 1.5, 2} {
		win := &scaleStartupWindow{}
		registerInitialInterfaceScale(win, scale)
		if win.minSizeCalls != 0 || win.zoomCalls != 0 {
			t.Fatal("startup called window setters before the UI loop could run")
		}
		if win.ready == nil {
			t.Fatal("startup scaling is not waiting for the window runtime")
		}
		win.ready(nil)
		if win.zoom != scale || win.minWidth != int(mainWindowMinWidth*scale) || win.minHeight != int(mainWindowMinHeight*scale) {
			t.Fatalf("scale %v: zoom=%v min=%dx%d", scale, win.zoom, win.minWidth, win.minHeight)
		}
		win.ready(nil)
		if win.minSizeCalls != 1 || win.zoomCalls != 1 {
			t.Fatal("runtime reload reapplied the startup scale")
		}
	}
}

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
