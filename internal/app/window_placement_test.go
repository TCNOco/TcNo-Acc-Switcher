package app

import (
	"testing"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestMainWindowOptionsRestoreSavedPlacement(t *testing.T) {
	settings := platform.AppSettings{WindowWidth: 1400, WindowHeight: 900, WindowX: 300, WindowY: 120}

	opts := mainWindowOptions(settings, cli.Parsed{})

	if opts.Width != 1400 || opts.Height != 900 {
		t.Fatalf("size = %dx%d, want 1400x900", opts.Width, opts.Height)
	}
	if opts.InitialPosition != application.WindowXY || opts.X != 300 || opts.Y != 120 {
		t.Fatalf("position = %v %d,%d", opts.InitialPosition, opts.X, opts.Y)
	}
}

func TestMainWindowOptionsWithoutSavedPlacement(t *testing.T) {
	opts := mainWindowOptions(platform.AppSettings{}, cli.Parsed{})

	if opts.Width != 0 || opts.Height != 0 {
		t.Fatalf("size = %dx%d, want the framework default", opts.Width, opts.Height)
	}
	if opts.X != 96 || opts.Y != 96 {
		t.Fatalf("position = %d,%d, want the fixed corner", opts.X, opts.Y)
	}
}

func TestMainWindowOptionsCentredKeepsSizeOnly(t *testing.T) {
	settings := platform.AppSettings{
		StartProgramCentered: true,
		WindowWidth:          1400,
		WindowHeight:         900,
		WindowX:              300,
		WindowY:              120,
	}

	opts := mainWindowOptions(settings, cli.Parsed{})

	if opts.Width != 1400 || opts.Height != 900 {
		t.Fatalf("size = %dx%d, want 1400x900", opts.Width, opts.Height)
	}
	if opts.InitialPosition != application.WindowCentered {
		t.Fatalf("InitialPosition = %v, want centered", opts.InitialPosition)
	}
}

func TestMainWindowOptionsClampSavedSizeToMinimum(t *testing.T) {
	settings := platform.AppSettings{WindowWidth: 200, WindowHeight: 100}

	opts := mainWindowOptions(settings, cli.Parsed{})

	if opts.Width != mainWindowMinWidth || opts.Height != mainWindowMinHeight {
		t.Fatalf("size = %dx%d, want the minimum %dx%d", opts.Width, opts.Height, mainWindowMinWidth, mainWindowMinHeight)
	}
}

// A saved placement must never become a StartState: the platform layer maximises
// before it applies X/Y, which loses the restored bounds.
func TestMainWindowOptionsLeaveMaximiseToRuntime(t *testing.T) {
	settings := platform.AppSettings{WindowWidth: 1400, WindowHeight: 900, WindowMaximised: true}

	opts := mainWindowOptions(settings, cli.Parsed{})

	if opts.StartState != application.WindowStateNormal {
		t.Fatalf("StartState = %v, want normal", opts.StartState)
	}
}

func TestFitWindowBounds(t *testing.T) {
	primary := application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}
	secondary := application.Rect{X: 1920, Y: 0, Width: 2560, Height: 1400}

	for _, tc := range []struct {
		name        string
		bounds      application.Rect
		areas       []application.Rect
		want        application.Rect
		wantChanged bool
	}{
		{
			name:   "already on a screen",
			bounds: application.Rect{X: 200, Y: 150, Width: 1200, Height: 800},
			areas:  []application.Rect{primary, secondary},
			want:   application.Rect{X: 200, Y: 150, Width: 1200, Height: 800},
		},
		{
			name:   "spanning two screens",
			bounds: application.Rect{X: 1700, Y: 100, Width: 1200, Height: 800},
			areas:  []application.Rect{primary, secondary},
			want:   application.Rect{X: 1700, Y: 100, Width: 1200, Height: 800},
		},
		{
			name:        "monitor unplugged",
			bounds:      application.Rect{X: 2400, Y: 300, Width: 1200, Height: 800},
			areas:       []application.Rect{primary},
			want:        application.Rect{X: 360, Y: 120, Width: 1200, Height: 800},
			wantChanged: true,
		},
		{
			name:        "hanging off the right edge",
			bounds:      application.Rect{X: 1880, Y: 200, Width: 1200, Height: 800},
			areas:       []application.Rect{primary},
			want:        application.Rect{X: 1760, Y: 200, Width: 1200, Height: 800},
			wantChanged: true,
		},
		{
			name:        "title bar above the work area",
			bounds:      application.Rect{X: 200, Y: -300, Width: 1200, Height: 800},
			areas:       []application.Rect{primary},
			want:        application.Rect{X: 200, Y: 0, Width: 1200, Height: 800},
			wantChanged: true,
		},
		{
			name:        "sized for a bigger screen",
			bounds:      application.Rect{X: 0, Y: 0, Width: 2400, Height: 1300},
			areas:       []application.Rect{primary},
			want:        application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040},
			wantChanged: true,
		},
		{
			name:   "no screens known",
			bounds: application.Rect{X: 5000, Y: 5000, Width: 1200, Height: 800},
			areas:  nil,
			want:   application.Rect{X: 5000, Y: 5000, Width: 1200, Height: 800},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := fitWindowBounds(tc.bounds, tc.areas, mainWindowMinWidth, mainWindowMinHeight)
			if got != tc.want || changed != tc.wantChanged {
				t.Fatalf("fitWindowBounds = %+v, %v; want %+v, %v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

type fakeWindowGeometry struct {
	bounds     application.Rect
	minimised  bool
	maximised  bool
	fullscreen bool
}

func (f *fakeWindowGeometry) Bounds() application.Rect { return f.bounds }
func (f *fakeWindowGeometry) IsMinimised() bool        { return f.minimised }
func (f *fakeWindowGeometry) IsMaximised() bool        { return f.maximised }
func (f *fakeWindowGeometry) IsFullscreen() bool       { return f.fullscreen }

func newTestRecorder(win *fakeWindowGeometry, last platform.WindowPlacement) (*windowPlacementRecorder, *[]platform.WindowPlacement) {
	var saved []platform.WindowPlacement
	rec := &windowPlacementRecorder{
		win:  win,
		last: last,
		save: func(p platform.WindowPlacement) error {
			saved = append(saved, p)
			return nil
		},
	}
	return rec, &saved
}

func TestRecorderStoresNormalBounds(t *testing.T) {
	win := &fakeWindowGeometry{bounds: application.Rect{X: 40, Y: 60, Width: 1000, Height: 700}}
	rec, saved := newTestRecorder(win, platform.WindowPlacement{})

	rec.flush()

	want := platform.WindowPlacement{Width: 1000, Height: 700, X: 40, Y: 60}
	if len(*saved) != 1 || (*saved)[0] != want {
		t.Fatalf("saved = %+v, want one %+v", *saved, want)
	}
}

func TestRecorderKeepsRestoredSizeWhileMaximised(t *testing.T) {
	restored := platform.WindowPlacement{Width: 1000, Height: 700, X: 40, Y: 60}
	win := &fakeWindowGeometry{bounds: application.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}, maximised: true}
	rec, saved := newTestRecorder(win, restored)

	rec.flush()

	want := restored
	want.Maximised = true
	if len(*saved) != 1 || (*saved)[0] != want {
		t.Fatalf("saved = %+v, want one %+v", *saved, want)
	}
}

func TestRecorderSkipsUnusableStates(t *testing.T) {
	for _, tc := range []struct {
		name string
		win  *fakeWindowGeometry
	}{
		{"minimised", &fakeWindowGeometry{bounds: application.Rect{X: -32000, Y: -32000, Width: 160, Height: 28}, minimised: true}},
		{"fullscreen", &fakeWindowGeometry{bounds: application.Rect{Width: 1920, Height: 1080}, fullscreen: true}},
		{"maximised before anything was recorded", &fakeWindowGeometry{bounds: application.Rect{Width: 1920, Height: 1040}, maximised: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, saved := newTestRecorder(tc.win, platform.WindowPlacement{})
			rec.flush()
			if len(*saved) != 0 {
				t.Fatalf("saved %+v, want nothing", *saved)
			}
		})
	}
}

func TestRecorderDoesNotRewriteUnchangedPlacement(t *testing.T) {
	placement := platform.WindowPlacement{Width: 1000, Height: 700, X: 40, Y: 60}
	win := &fakeWindowGeometry{bounds: application.Rect{X: 40, Y: 60, Width: 1000, Height: 700}}
	rec, saved := newTestRecorder(win, placement)

	rec.flush()
	rec.flush()

	if len(*saved) != 0 {
		t.Fatalf("saved %+v, want nothing", *saved)
	}
}

func TestRecorderStopsWriting(t *testing.T) {
	win := &fakeWindowGeometry{bounds: application.Rect{X: 40, Y: 60, Width: 1000, Height: 700}}
	rec, saved := newTestRecorder(win, platform.WindowPlacement{})

	rec.stop()
	win.bounds = application.Rect{X: 10, Y: 10, Width: 900, Height: 600}
	rec.schedule()
	rec.flush()

	if len(*saved) != 1 {
		t.Fatalf("saved = %+v, want only the final flush", *saved)
	}
}
