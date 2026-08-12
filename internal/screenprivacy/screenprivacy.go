// Package screenprivacy keeps app windows out of screen captures on demand.
//
// It is the same mechanism the Steam Guard windows already use — Windows'
// SetWindowDisplayAffinity with WDA_EXCLUDEFROMCAPTURE, reached through Wails'
// ContentProtectionEnabled — applied to the ordinary windows for as long as the
// user asks for it. Captures see a blank rectangle where the window is; the
// window itself looks normal on the physical screen.
//
// Windows opt in with Follow. Anything that protects itself unconditionally, like
// the Steam Guard confirmations window, must stay out: this package would switch
// that protection off the moment the preference was turned off.
package screenprivacy

import (
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	enabled atomic.Bool

	mu       sync.Mutex
	followed = map[uint]application.Window{}
)

// Enabled reports whether followed windows are currently hidden from capture.
func Enabled() bool {
	return enabled.Load()
}

// Apply stamps the preference onto options for a window that has not been created
// yet, so it is protected from its first frame rather than from the first toggle.
// It never clears the flag: a window that asked for protection itself keeps it.
func Apply(options *application.WebviewWindowOptions) {
	if options == nil {
		return
	}
	if enabled.Load() {
		options.ContentProtectionEnabled = true
	}
}

// Follow registers a window as tracking the preference for the rest of its life.
//
// Registration only: the window's starting state comes from Apply, and calling
// SetContentProtection here would deadlock. Both call sites create their window
// on the main thread — the main window before Run() is pumping at all — while
// SetContentProtection dispatches to the main thread and blocks until it answers.
func Follow(window application.Window) {
	if window == nil {
		return
	}
	mu.Lock()
	followed[window.ID()] = window
	mu.Unlock()
}

// SetEnabled records the preference and applies it to every followed window.
func SetEnabled(on bool) {
	enabled.Store(on)
	for _, window := range liveFollowed() {
		window.SetContentProtection(on)
	}
}

// liveFollowed returns the followed windows that still exist, dropping the rest.
// Pruning here rather than on a close event keeps the registry to one code path:
// windows are only ever consulted when the preference changes, which is rare.
func liveFollowed() []application.Window {
	app := application.Get()
	if app == nil {
		return nil
	}
	alive := make(map[uint]struct{})
	for _, window := range app.Window.GetAll() {
		alive[window.ID()] = struct{}{}
	}

	mu.Lock()
	defer mu.Unlock()
	out := make([]application.Window, 0, len(followed))
	for id, window := range followed {
		if _, ok := alive[id]; !ok {
			delete(followed, id)
			continue
		}
		out = append(out, window)
	}
	return out
}
