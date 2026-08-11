//go:build windows

package streamer

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/winutil"

	"golang.org/x/sys/windows"
)

// Detection never polls the process table on a timer.
//
// Start: two WinEvent hooks. Windows delivers a message only when a window is shown
// or takes the foreground, so an idle machine costs literally nothing. Each event is
// a PID lookup against a bounded map; only a PID we have not classified yet is worth
// an OpenProcess to read its image name.
//
// Stop: one goroutine per matched process, parked in WaitForSingleObject(INFINITE).
// The kernel wakes it at exit. No timers, no retries, no wasted wakeups.
//
// The one gap either approach shares is a broadcaster that starts with no window at
// all; a broadcaster already running when the watcher starts is covered by the single
// snapshot pass below.
const (
	eventSystemForeground = 0x0003
	eventObjectShow       = 0x8002

	winEventOutOfContext   = 0x0000
	winEventSkipOwnProcess = 0x0002

	objidWindow = 0
	childidSelf = 0
	gaRoot      = 2

	wmQuit = 0x0012

	// seenCap bounds the classified-PID map. Windows recycles PIDs, so this is a
	// cache and not a ledger: dropping it costs one extra image-name lookup.
	seenCap = 4096
)

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procSetWinEventHook          = modUser32.NewProc("SetWinEventHook")
	procUnhookWinEvent           = modUser32.NewProc("UnhookWinEvent")
	procGetMessageW              = modUser32.NewProc("GetMessageW")
	procTranslateMessage         = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW         = modUser32.NewProc("DispatchMessageW")
	procPostThreadMessageW       = modUser32.NewProc("PostThreadMessageW")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetAncestor              = modUser32.NewProc("GetAncestor")

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
	private uint32
}

type watcher struct {
	mu       sync.Mutex
	running  bool
	threadID uint32
	targets  map[string]struct{}
	// seen maps PID to "is a broadcaster", so repeat events for ordinary windows
	// never reach OpenProcess.
	seen map[uint32]bool
	// live holds matched processes that are still alive, keyed by PID.
	live map[uint32]string
	// generation invalidates in-flight exit waiters after a stop/start cycle.
	generation uint64
}

var (
	watch        watcher
	callbackOnce sync.Once
	callbackPtr  uintptr
)

// setWatching starts or stops detection. Idempotent; called on every state change.
func setWatching(on bool) {
	watch.mu.Lock()
	if on == watch.running {
		watch.mu.Unlock()
		return
	}
	watch.running = on
	watch.generation++
	generation := watch.generation
	threadID := watch.threadID
	if on {
		watch.targets = targetSet()
		watch.seen = make(map[uint32]bool)
		watch.live = make(map[uint32]string)
	} else {
		watch.targets = nil
		watch.seen = nil
		watch.live = nil
		watch.threadID = 0
	}
	watch.mu.Unlock()

	if !on {
		if threadID != 0 {
			procPostThreadMessageW.Call(uintptr(threadID), wmQuit, 0, 0)
		}
		return
	}

	go func() {
		defer crashlog.Capture()
		watch.runMessageLoop(generation)
	}()
	go func() {
		defer crashlog.Capture()
		watch.adoptRunning(generation)
	}()
}

// adoptRunning is the only full pass over the process table: broadcasters that were
// already open when detection started have no window event left to fire.
func (w *watcher) adoptRunning(generation uint64) {
	w.mu.Lock()
	targets := w.targets
	w.mu.Unlock()
	if targets == nil {
		return
	}
	found, err := winutil.SnapshotMatchingPIDs(targets)
	if err != nil {
		return
	}
	for pid, exe := range found {
		w.track(pid, exe, generation)
	}
}

func (w *watcher) runMessageLoop(generation uint64) {
	// WinEvent hooks are owned by the thread that installs them and their callbacks
	// arrive through that thread's message queue, so it must not be rescheduled.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	threadID, _, _ := procGetCurrentThreadId.Call()
	w.mu.Lock()
	if w.generation != generation {
		w.mu.Unlock()
		return
	}
	w.threadID = uint32(threadID)
	w.mu.Unlock()

	// NewCallback allocates from a process-wide table that is never freed, so the
	// trampoline is built once no matter how often detection is toggled.
	callbackOnce.Do(func() { callbackPtr = windows.NewCallback(winEventProc) })

	var hooks []uintptr
	for _, event := range []uint32{eventSystemForeground, eventObjectShow} {
		h, _, _ := procSetWinEventHook.Call(
			uintptr(event), uintptr(event),
			0, callbackPtr, 0, 0,
			winEventOutOfContext|winEventSkipOwnProcess,
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

// winEventProc runs on the watcher thread for every hooked event. Everything before
// the seen-map check is a handful of instructions, which is the point.
func winEventProc(_ uintptr, _ uint32, hwnd uintptr, idObject, idChild int32, _, _ uint32) uintptr {
	if hwnd == 0 || idObject != objidWindow || idChild != childidSelf {
		return 0
	}
	if root, _, _ := procGetAncestor.Call(hwnd, gaRoot); root != hwnd {
		return 0
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return 0
	}
	watch.consider(pid)
	return 0
}

func (w *watcher) consider(pid uint32) {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	if _, classified := w.seen[pid]; classified {
		w.mu.Unlock()
		return
	}
	if len(w.seen) >= seenCap {
		w.seen = make(map[uint32]bool)
	}
	w.seen[pid] = false
	targets := w.targets
	generation := w.generation
	w.mu.Unlock()

	exe := strings.ToLower(processImageBase(pid))
	if exe == "" {
		return
	}
	if _, ok := targets[exe]; !ok {
		return
	}
	w.track(pid, exe, generation)
}

// track registers a live broadcaster and parks a goroutine on its exit.
func (w *watcher) track(pid uint32, exe string, generation uint64) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		// Without a waitable handle there is no exit signal, and a stuck "streaming"
		// state is worse than a missed detection.
		return
	}

	w.mu.Lock()
	if !w.running || w.generation != generation {
		w.mu.Unlock()
		windows.CloseHandle(handle)
		return
	}
	if _, dup := w.live[pid]; dup {
		w.mu.Unlock()
		windows.CloseHandle(handle)
		return
	}
	w.seen[pid] = true
	w.live[pid] = exe
	first := len(w.live) == 1
	w.mu.Unlock()

	if first {
		setDetected(true, exe)
	}

	go func() {
		defer crashlog.Capture()
		defer windows.CloseHandle(handle)
		_, _ = windows.WaitForSingleObject(handle, windows.INFINITE)

		w.mu.Lock()
		if w.generation != generation {
			w.mu.Unlock()
			return
		}
		delete(w.live, pid)
		delete(w.seen, pid)
		remaining := ""
		for _, name := range w.live {
			remaining = name
			break
		}
		empty := len(w.live) == 0
		w.mu.Unlock()

		if empty {
			setDetected(false, "")
		} else {
			setDetected(true, remaining)
		}
	}()
}

func processImageBase(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:size]))
}
