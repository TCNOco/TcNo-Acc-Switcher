// Package foreground reports when another application is covering the whole
// screen, so the UI can stop animating things nobody can see.
//
// An account switcher spends most of its life behind a game. The account list
// draws Steam's animated avatar frames, which are multi-megabyte APNGs and GIFs
// that keep decoding and compositing for as long as they are in the document —
// roughly a quarter of a core, whether or not a pixel of the window is visible.
// Suspending them while a fullscreen application is in front costs the user
// nothing: the frames are already hidden behind the game.
package foreground

// Rect is a screen rectangle in virtual-desktop coordinates, matching Win32's
// RECT: Right and Bottom are exclusive.
type Rect struct {
	Left, Top, Right, Bottom int32
}

// Empty reports whether r has no area, which is how Win32 describes a window
// that is minimised or has not been laid out yet.
func (r Rect) Empty() bool {
	return r.Right <= r.Left || r.Bottom <= r.Top
}

// CoversMonitor reports whether win hides every pixel of mon.
//
// Exclusive fullscreen and borderless fullscreen both land here: the first
// matches the monitor exactly, the second is a plain window sized to it. Windows
// that spill past the monitor edge still count, since nothing of the monitor
// shows through.
func CoversMonitor(win, mon Rect) bool {
	if win.Empty() || mon.Empty() {
		return false
	}
	return win.Left <= mon.Left &&
		win.Top <= mon.Top &&
		win.Right >= mon.Right &&
		win.Bottom >= mon.Bottom
}

// Win32 window styles used to tell a fullscreen application from an ordinary
// window that merely happens to be monitor-sized.
const (
	WSMaximize   = 0x01000000
	WSCaption    = 0x00C00000
	WSThickFrame = 0x00040000
)

// LooksFullscreen reports whether a window covering its monitor is a fullscreen
// application rather than a maximised ordinary one.
//
// Covering the monitor is not enough on its own. A maximised window reaches the
// full monitor rect whenever the taskbar is on another screen — and Windows
// sizes it a few pixels beyond each edge for the invisible resize border, so it
// overshoots exactly like a game does. A maximised browser on a second monitor
// would otherwise read as a game for as long as it was focused.
//
// Games are borderless: exclusive fullscreen and borderless fullscreen both drop
// the caption and the sizing frame, and neither sets the maximised state.
func LooksFullscreen(style uint32, win, mon Rect) bool {
	if !CoversMonitor(win, mon) {
		return false
	}
	if style&WSMaximize != 0 {
		return false
	}
	return style&(WSCaption|WSThickFrame) == 0
}

// shellClasses are the desktop and taskbar. The desktop is monitor-sized by
// definition, so without this it would read as a fullscreen application every
// time the user minimises everything.
var shellClasses = map[string]struct{}{
	"Progman":                    {},
	"WorkerW":                    {},
	"Shell_TrayWnd":              {},
	"Shell_SecondaryTrayWnd":     {},
	"DV2ControlHost":             {},
	"Windows.UI.Core.CoreWindow": {},
}

// IsShellClass reports whether a window class belongs to the Windows shell
// rather than to an application.
func IsShellClass(class string) bool {
	_, ok := shellClasses[class]
	return ok
}
