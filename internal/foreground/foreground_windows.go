//go:build windows

package foreground

import (
	"runtime"
	"sync"
	"unsafe"

	"TcNo-Acc-Switcher/internal/crashlog"

	"golang.org/x/sys/windows"
)

// Detection is event-driven, matching the streamer watcher: WinEvent hooks for
// foreground changes, minimise/restore, and window moves. An idle machine costs
// nothing, and a game taking the screen is a single callback.
const (
	eventSystemForeground     = 0x0003
	eventSystemMinimizeStart  = 0x0016
	eventSystemMinimizeEnd    = 0x0017
	eventObjectLocationChange = 0x800B

	winEventOutOfContext = 0x0000

	monitorDefaultToNearest = 0x0002

	// OBJID_WINDOW: the window itself rather than one of its child objects.
	objidWindow = 0

	wmQuit = 0x0012
)

// GWL_STYLE. A variable rather than a const because a negative untyped constant
// cannot be converted to uintptr at compile time.
var gwlStyle = int32(-16)

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procSetWinEventHook          = modUser32.NewProc("SetWinEventHook")
	procUnhookWinEvent           = modUser32.NewProc("UnhookWinEvent")
	procGetMessageW              = modUser32.NewProc("GetMessageW")
	procTranslateMessage         = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW         = modUser32.NewProc("DispatchMessageW")
	procPostThreadMessageW       = modUser32.NewProc("PostThreadMessageW")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindowRect            = modUser32.NewProc("GetWindowRect")
	procGetClassNameW            = modUser32.NewProc("GetClassNameW")
	procMonitorFromWindow        = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW          = modUser32.NewProc("GetMonitorInfoW")
	procGetWindowLongW           = modUser32.NewProc("GetWindowLongPtrW")

	modKernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId = modKernel32.NewProc("GetCurrentThreadId")
)

type msgStruct struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type monitorInfo struct {
	cbSize    uint32
	rcMonitor Rect
	rcWork    Rect
	dwFlags   uint32
}

type watcher struct {
	mu       sync.Mutex
	running  bool
	threadID uint32
	covered  bool
	onChange func(bool)
}

var (
	watch        watcher
	callbackOnce sync.Once
	callbackPtr  uintptr
	ownPID       = uint32(windows.Getpid())
)

// Watch reports whether a fullscreen application from another process is in
// front. onChange fires on transitions only, starting with the state at the
// moment Watch is called. The returned func stops the hooks.
func Watch(onChange func(covered bool)) func() {
	if onChange == nil {
		return func() {}
	}

	watch.mu.Lock()
	if watch.running {
		watch.mu.Unlock()
		return func() {}
	}
	watch.running = true
	watch.onChange = onChange
	watch.covered = fullscreenForeign()
	initial := watch.covered
	watch.mu.Unlock()

	onChange(initial)

	go func() {
		defer crashlog.Capture()
		watch.runMessageLoop()
	}()

	return func() {
		watch.mu.Lock()
		if !watch.running {
			watch.mu.Unlock()
			return
		}
		watch.running = false
		threadID := watch.threadID
		watch.threadID = 0
		watch.onChange = nil
		watch.mu.Unlock()
		if threadID != 0 {
			procPostThreadMessageW.Call(uintptr(threadID), wmQuit, 0, 0)
		}
	}
}

func (w *watcher) runMessageLoop() {
	// The hook belongs to the thread that installs it and its callbacks arrive
	// through that thread's message queue, so the thread must not be rescheduled.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid, _, _ := procGetCurrentThreadId.Call()
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.threadID = uint32(tid)
	w.mu.Unlock()

	callbackOnce.Do(func() {
		callbackPtr = windows.NewCallback(func(hook, event, hwnd, idObject, idChild, thread, ts uintptr) uintptr {
			// A window can take the foreground and only then size itself to the
			// screen — which is exactly what a game does when you switch it to
			// fullscreen, and what one doing its own startup sizing looks like.
			// Foreground events alone therefore miss it, so resize events count
			// too; they are filtered to the window that already has focus so the
			// firehose of location changes from every other window is ignored.
			if uint32(event) == eventObjectLocationChange {
				if idObject != objidWindow {
					return 0
				}
				fg, _, _ := procGetForegroundWindow.Call()
				if fg == 0 || fg != hwnd {
					return 0
				}
			}
			watch.reevaluate()
			return 0
		})
	})

	hooks := make([]uintptr, 0, 3)
	for _, ev := range [][2]uint32{
		{eventSystemForeground, eventSystemForeground},
		{eventSystemMinimizeStart, eventSystemMinimizeEnd},
		{eventObjectLocationChange, eventObjectLocationChange},
	} {
		h, _, _ := procSetWinEventHook.Call(
			uintptr(ev[0]), uintptr(ev[1]), 0, callbackPtr, 0, 0, winEventOutOfContext,
		)
		if h != 0 {
			hooks = append(hooks, h)
		}
	}
	defer func() {
		for _, h := range hooks {
			procUnhookWinEvent.Call(h)
		}
	}()

	var msg msgStruct
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// reevaluate recomputes the state and notifies only when it flips.
func (w *watcher) reevaluate() {
	next := fullscreenForeign()

	w.mu.Lock()
	if !w.running || next == w.covered {
		w.mu.Unlock()
		return
	}
	w.covered = next
	cb := w.onChange
	w.mu.Unlock()

	if cb != nil {
		cb(next)
	}
}

// fullscreenForeign reports whether the foreground window belongs to another
// process and covers its entire monitor.
func fullscreenForeign() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	// Our own window filling the screen is the user looking at us, not at a game.
	if pid == 0 || pid == ownPID {
		return false
	}

	if IsShellClass(classNameOf(hwnd)) {
		return false
	}

	win, ok := windowRect(hwnd)
	if !ok {
		return false
	}
	mon, ok := monitorRect(hwnd)
	if !ok {
		return false
	}
	return LooksFullscreen(windowStyle(hwnd), win, mon)
}

func windowStyle(hwnd uintptr) uint32 {
	style, _, _ := procGetWindowLongW.Call(hwnd, uintptr(gwlStyle))
	return uint32(style)
}

func classNameOf(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func windowRect(hwnd uintptr) (Rect, bool) {
	var r Rect
	ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r, ok != 0
}

func monitorRect(hwnd uintptr) (Rect, bool) {
	mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if mon == 0 {
		return Rect{}, false
	}
	mi := monitorInfo{cbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	ok, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	if ok == 0 {
		return Rect{}, false
	}
	return mi.rcMonitor, true
}
