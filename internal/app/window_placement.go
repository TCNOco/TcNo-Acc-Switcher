package app

import (
	"log/slog"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	// A drag emits a geometry event per frame and every write is a whole
	// settings file, so the window has to sit still before one is made.
	windowPlacementSettleDelay = 400 * time.Millisecond

	// The smallest slice of window that has to land on a screen for a remembered
	// position to still be usable: roughly the drag strip of the custom title
	// bar. Below this there is nothing left to grab the window by.
	minVisibleWindowWidth  = 160
	minVisibleWindowHeight = 32
)

// applySavedWindowPlacement puts the remembered size and position into the
// options the main window is built from.
//
// The maximised flag is deliberately not turned into a StartState: the platform
// layer maximises before it applies X/Y, leaving no restored bounds underneath.
// registerWindowPlacement re-applies it once the window exists instead.
func applySavedWindowPlacement(opts *application.WebviewWindowOptions, saved platform.WindowPlacement, centered bool) {
	if opts == nil || !saved.HasSize() {
		return
	}
	opts.Width = max(saved.Width, opts.MinWidth)
	opts.Height = max(saved.Height, opts.MinHeight)
	if centered {
		return
	}
	opts.InitialPosition = application.WindowXY
	opts.X = saved.X
	opts.Y = saved.Y
}

// registerWindowPlacement makes the main window reopen where it was left: it
// records geometry as the user changes it, and finishes the restore that window
// creation could not do on its own.
func registerWindowPlacement(app *application.App, win *application.WebviewWindow, svc *platform.PlatformService, saved platform.WindowPlacement) {
	if app == nil || win == nil || svc == nil {
		return
	}
	rec := &windowPlacementRecorder{
		win:  win,
		save: func(p platform.WindowPlacement) error { return platform.SaveWindowPlacement(svc, p) },
		log:  app.Logger,
		last: saved,
	}
	for _, evt := range []events.WindowEventType{events.Common.WindowDidResize, events.Common.WindowDidMove} {
		win.OnWindowEvent(evt, func(*application.WindowEvent) { rec.schedule() })
	}
	// A hook rather than a listener, and registered before the tray's, so the
	// last drag is still recorded when closing to tray cancels the close.
	win.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) { rec.flush() })
	app.OnShutdown(rec.stop)

	// Both signals, because neither is guaranteed to be the first to arrive: a
	// window can be shown before its frontend has reported in, and one that
	// starts hidden reports in without being shown at all.
	finish := newPlacementFinisher(app, win, saved)
	for _, evt := range []events.WindowEventType{events.Common.WindowShow, events.Common.WindowRuntimeReady} {
		win.OnWindowEvent(evt, func(*application.WindowEvent) { finish() })
	}
}

// newPlacementFinisher returns the half of the restore that needs a platform
// window to exist, which it does not at window creation nor at
// ApplicationStarted: until then the screen list is empty, Bounds() reads back
// zero, and Maximise() only records a start state applied before the saved X/Y.
//
// Idempotent rather than once-only: the events it hangs off can arrive while
// creation is still in flight, and a call that finds nothing to read has to
// leave the job for the next one.
func newPlacementFinisher(app *application.App, win *application.WebviewWindow, saved platform.WindowPlacement) func() {
	var (
		mu     sync.Mutex
		fitted bool
		zoomed bool
	)
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if saved.HasSize() && !fitted {
			fitted = fitWindowToScreens(app, win)
		}
		// Only once something has put the window on screen: maximising a window
		// that started in the tray is the one thing a tray start asks not to
		// happen.
		if saved.Maximised && !zoomed && win.IsVisible() {
			win.Maximise()
			zoomed = true
		}
	}
}

// fitWindowToScreens pulls the window back onto the desktop when the monitor it
// was left on is gone, or is smaller than the one it was sized for. It reports
// false when the screens or the window's bounds are not readable yet, leaving
// the check for a later caller.
func fitWindowToScreens(app *application.App, win *application.WebviewWindow) bool {
	areas := screenWorkAreas(app)
	if len(areas) == 0 {
		return false
	}
	bounds := win.Bounds()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return false
	}
	if fitted, changed := fitWindowBounds(bounds, areas, mainWindowMinWidth, mainWindowMinHeight); changed {
		win.SetBounds(fitted)
	}
	return true
}

func screenWorkAreas(app *application.App) []application.Rect {
	if app == nil || app.Screen == nil {
		return nil
	}
	var areas []application.Rect
	for _, screen := range app.Screen.GetAll() {
		if screen == nil || screen.WorkArea.Width <= 0 || screen.WorkArea.Height <= 0 {
			continue
		}
		areas = append(areas, screen.WorkArea)
	}
	return areas
}

// fitWindowBounds returns bounds trimmed to the work area they mostly sit on and
// nudged far enough into it to stay reachable. changed is false when the window
// is already somewhere usable, which is the normal case.
func fitWindowBounds(bounds application.Rect, areas []application.Rect, minWidth, minHeight int) (application.Rect, bool) {
	if len(areas) == 0 {
		return bounds, false
	}
	target := areas[0]
	overlap := 0
	for _, area := range areas {
		shared := intersectRects(bounds, area)
		if size := shared.Width * shared.Height; size > overlap {
			overlap, target = size, area
		}
	}

	fitted := bounds
	fitted.Width = min(bounds.Width, max(target.Width, minWidth))
	fitted.Height = min(bounds.Height, max(target.Height, minHeight))
	if overlap == 0 {
		// None of the window is on a screen any more, so nothing about the old
		// position is worth keeping.
		fitted.X = target.X + (target.Width-fitted.Width)/2
		fitted.Y = target.Y + (target.Height-fitted.Height)/2
		return fitted, fitted != bounds
	}
	fitted.X = clampInt(fitted.X, target.X+minVisibleWindowWidth-fitted.Width, target.X+target.Width-minVisibleWindowWidth)
	// Never above the work area: a title bar under the top edge cannot be dragged.
	fitted.Y = clampInt(fitted.Y, target.Y, target.Y+target.Height-minVisibleWindowHeight)
	return fitted, fitted != bounds
}

func intersectRects(a, b application.Rect) application.Rect {
	x := max(a.X, b.X)
	y := max(a.Y, b.Y)
	right := min(a.X+a.Width, b.X+b.Width)
	bottom := min(a.Y+a.Height, b.Y+b.Height)
	if right <= x || bottom <= y {
		return application.Rect{}
	}
	return application.Rect{X: x, Y: y, Width: right - x, Height: bottom - y}
}

func clampInt(value, low, high int) int {
	if low > high {
		return low
	}
	return min(max(value, low), high)
}

// windowGeometry is the part of the main window a recorder reads, kept as an
// interface so the rules below can be exercised without one.
type windowGeometry interface {
	Bounds() application.Rect
	IsMinimised() bool
	IsMaximised() bool
	IsFullscreen() bool
}

// windowPlacementRecorder writes the main window's geometry to settings once the
// user stops changing it.
type windowPlacementRecorder struct {
	win  windowGeometry
	save func(platform.WindowPlacement) error
	log  *slog.Logger

	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
	last    platform.WindowPlacement
}

func (r *windowPlacementRecorder) schedule() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	if r.timer == nil {
		r.timer = time.AfterFunc(windowPlacementSettleDelay, r.flush)
		return
	}
	r.timer.Reset(windowPlacementSettleDelay)
}

// flush records the window's current geometry. Reading it here rather than in
// the event handler keeps a whole drag down to one main-thread round trip.
func (r *windowPlacementRecorder) flush() {
	r.mu.Lock()
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	stopped := r.stopped
	r.mu.Unlock()
	if stopped {
		return
	}

	placement, ok := r.capture()
	if !ok {
		return
	}
	r.mu.Lock()
	unchanged := placement == r.last
	r.last = placement
	r.mu.Unlock()
	if unchanged {
		return
	}
	if err := r.save(placement); err != nil && r.log != nil {
		r.log.Warn("window placement save", "error", err)
	}
}

func (r *windowPlacementRecorder) stop() {
	r.flush()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func (r *windowPlacementRecorder) capture() (platform.WindowPlacement, bool) {
	win := r.win
	if win == nil || r.save == nil || win.IsMinimised() || win.IsFullscreen() {
		return platform.WindowPlacement{}, false
	}
	if win.IsMaximised() {
		// A maximised window reports the screen's bounds. Storing those as the
		// restored size would make un-maximising a no-op on the next launch, so
		// only the flag moves.
		r.mu.Lock()
		current := r.last
		r.mu.Unlock()
		current.Maximised = true
		return current, current.HasSize()
	}
	bounds := win.Bounds()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return platform.WindowPlacement{}, false
	}
	return platform.WindowPlacement{
		Width:  bounds.Width,
		Height: bounds.Height,
		X:      bounds.X,
		Y:      bounds.Y,
	}, true
}
