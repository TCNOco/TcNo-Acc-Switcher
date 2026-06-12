//go:build windows

package winutil

import (
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const chromeWidgetWinClass = "Chrome_WidgetWin_1"

const (
	winKEYEVENTFKeyUp = 0x0002
	winVKMENU         = 0x12
	winVKLMENU        = 0xA4
	winVKRMENU        = 0xA5
	winVKF4           = 0x73
	winINPUTKeyboard  = 1
	winSWRestore      = 9
	winSWShow         = 5

	electronSynthFocusAttempts    = 12
	electronSynthFocusPoll        = 50 * time.Millisecond
	electronSynthTrayRecoverPause = 950 * time.Millisecond
)

// electronSynthExitMu prevents overlapping AttachThreadInput / synth SendInput sequences.
var electronSynthExitMu sync.Mutex

// sendInputRecord is one INPUT for SendInput: MSVC x64 pads the union to 40 bytes.
type sendInputRecord struct {
	Type uint32
	_    uint32
	Ki   struct {
		WVk         uint16
		WScan       uint16
		DwFlags     uint32
		Time        uint32
		DwExtraInfo uintptr
		_           [8]byte
	}
}

type attachThreadGuard struct {
	cur          uint32
	fgThread     uint32
	targThread   uint32
	okFG, okTarg bool
}

func (g *attachThreadGuard) detach() {
	c := uintptr(g.cur)
	if g.okTarg {
		procAttachThreadInput.Call(uintptr(g.targThread), c, 0)
		g.okTarg = false
	}
	if g.okFG {
		procAttachThreadInput.Call(uintptr(g.fgThread), c, 0)
		g.okFG = false
	}
}

func windowClassName(hwnd uintptr) string {
	var buf [256]uint16
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	ln := int(n)
	if ln > len(buf) {
		ln = len(buf)
	}
	return windows.UTF16ToString(buf[:ln])
}

func enumTopLevelCallback(hwnd, _ uintptr) uintptr {
	s := &enumTopLevelState
	var windowPID uint32
	r0, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
	if r0 == 0 {
		return 1
	}
	if windowPID != s.pid {
		return 1
	}
	owner, _, _ := procGetWindow.Call(hwnd, uintptr(winGWOwner))
	if owner != 0 {
		return 1
	}
	if s.chromeOnly {
		if windowClassName(hwnd) != chromeWidgetWinClass {
			return 1
		}
	}
	*s.out = append(*s.out, windows.HWND(hwnd))
	return 1
}

func appendTopLevelHWNDsForPID(pid uint32, list *[]windows.HWND, chromeOnly bool) {
	if err := procEnumWindows.Find(); err != nil {
		return
	}
	enumTopLevelMu.Lock()
	defer enumTopLevelMu.Unlock()

	enumTopLevelState.pid = pid
	enumTopLevelState.chromeOnly = chromeOnly
	enumTopLevelState.out = list
	*list = (*list)[:0]

	enumTopLevelCb.Do(func() {
		enumTopLevelCb.ptr = syscall.NewCallback(enumTopLevelCallback)
	})
	_, _, _ = procEnumWindows.Call(enumTopLevelCb.ptr, 0)
}

func windowThreadID(hwnd windows.HWND) uint32 {
	t, _, _ := procGetWindowThreadProcessId.Call(uintptr(hwnd), 0)
	return uint32(t)
}

func hwndOwningPID(hwnd windows.HWND) uint32 {
	if hwnd == 0 {
		return 0
	}
	var pdf uint32
	r, _, _ := procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pdf)))
	if r == 0 {
		return 0
	}
	return pdf
}

func foregroundHWND() windows.HWND {
	fg, _, _ := procGetForegroundWindow.Call()
	return windows.HWND(fg)
}

func attachForegroundAndTarget(foregroundHWND, targetHWND windows.HWND, curThread uint32) attachThreadGuard {
	var g attachThreadGuard
	g.cur = curThread
	g.fgThread = windowThreadID(foregroundHWND)
	g.targThread = windowThreadID(targetHWND)

	if err := procAttachThreadInput.Find(); err != nil {
		return g
	}
	if g.fgThread != 0 && g.fgThread != g.cur {
		procAttachThreadInput.Call(uintptr(g.fgThread), uintptr(g.cur), 1)
		g.okFG = true
	}
	if g.targThread != g.cur && g.targThread != g.fgThread {
		procAttachThreadInput.Call(uintptr(g.targThread), uintptr(g.cur), 1)
		g.okTarg = true
	}
	return g
}

func isHWNDReallyVisible(hwnd windows.HWND) bool {
	if err := procIsWindowVisible.Find(); err != nil {
		return true
	}
	r, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return r != 0
}

func isHWNDIconic(hwnd windows.HWND) bool {
	if hwnd == 0 || procIsIconic.Find() != nil {
		return false
	}
	r, _, _ := procIsIconic.Call(uintptr(hwnd))
	return r != 0
}

func synthHWNDElectron(pid uint32) windows.HWND {
	var chrome []windows.HWND
	appendTopLevelHWNDsForPID(pid, &chrome, true)
	var hidden windows.HWND
	for _, h := range chrome {
		if isHWNDReallyVisible(h) {
			return h
		}
		if hidden == 0 {
			hidden = h
		}
	}
	if hidden != 0 {
		return hidden
	}
	var all []windows.HWND
	appendTopLevelHWNDsForPID(pid, &all, false)
	for _, h := range all {
		if isHWNDReallyVisible(h) {
			return h
		}
		if hidden == 0 {
			hidden = h
		}
	}
	return hidden
}

func sendAltF4SendInput() bool {
	if err := procSendInput.Find(); err != nil {
		return false
	}
	inpSz := uintptr(unsafe.Sizeof(sendInputRecord{}))
	if inpSz != 40 {
		log.Printf("winutil: SendInput INPUT sz=%d (expected 40 on windows/amd64); Alt+F4 may fail", inpSz)
	}
	var seq [4]sendInputRecord
	fill := func(i int, vk uint16, flags uint32) {
		seq[i].Type = winINPUTKeyboard
		seq[i].Ki.WVk = vk
		seq[i].Ki.DwFlags = flags
	}
	fill(0, winVKMENU, 0)
	fill(1, winVKF4, 0)
	fill(2, winVKF4, winKEYEVENTFKeyUp)
	fill(3, winVKMENU, winKEYEVENTFKeyUp)

	n, _, _ := procSendInput.Call(4, uintptr(unsafe.Pointer(&seq[0])), inpSz)
	return int32(n) == 4
}

func sendStaleModifierKeyUps() {
	if err := procSendInput.Find(); err != nil {
		return
	}
	inpSz := uintptr(unsafe.Sizeof(sendInputRecord{}))
	if inpSz != 40 {
		return
	}
	var seq [4]sendInputRecord
	fillUp := func(i int, vk uint16) {
		seq[i].Type = winINPUTKeyboard
		seq[i].Ki.WVk = vk
		seq[i].Ki.DwFlags = winKEYEVENTFKeyUp
	}
	fillUp(0, winVKMENU)
	fillUp(1, winVKLMENU)
	fillUp(2, winVKRMENU)
	fillUp(3, winVKF4)
	procSendInput.Call(4, uintptr(unsafe.Pointer(&seq[0])), inpSz)
}

func restoreHWNDForeground(previous windows.HWND) {
	if previous == 0 {
		return
	}
	if err := procSetForegroundWindow.Find(); err != nil {
		return
	}
	if err := procGetCurrentThreadId.Find(); err != nil {
		return
	}
	cur, _, _ := procGetForegroundWindow.Call()
	if windows.HWND(cur) == previous {
		return
	}
	curThr, _, _ := procGetCurrentThreadId.Call()
	g := attachForegroundAndTarget(windows.HWND(cur), previous, uint32(curThr))
	iconic, _, _ := procIsIconic.Call(uintptr(previous))
	if iconic != 0 {
		procShowWindow.Call(uintptr(previous), uintptr(winSWRestore))
	}
	procBringWindowToTop.Call(uintptr(previous))
	procSetForegroundWindow.Call(uintptr(previous))
	time.Sleep(20 * time.Millisecond)
	g.detach()
}

func synthElectronAltF4ForPID(pid uint32) bool {
	myPID := windows.GetCurrentProcessId()
	hwnd := synthHWNDElectron(pid)
	if hwnd == 0 {
		log.Printf("winutil: electron synth Alt+F4 skipped pid=%d (no HWND)", pid)
		return false
	}
	if err := procGetForegroundWindow.Find(); err != nil {
		return false
	}
	fgPrior, _, _ := procGetForegroundWindow.Call()

	if err := procGetCurrentThreadId.Find(); err != nil {
		return false
	}
	curT, _, _ := procGetCurrentThreadId.Call()
	cur := uint32(curT)

	_ = procSetForegroundWindow.Find()
	_ = procShowWindow.Find()
	_ = procBringWindowToTop.Find()

	g := attachForegroundAndTarget(windows.HWND(fgPrior), hwnd, cur)
	defer g.detach()

	_ = procIsIconic.Find()

	needsTrayRecoverPause := isHWNDIconic(hwnd) || !isHWNDReallyVisible(hwnd)
	trayRecoverWaitDone := false

	sendOK := false
	for attempt := 0; attempt < electronSynthFocusAttempts; attempt++ {
		iconic, _, _ := procIsIconic.Call(uintptr(hwnd))
		if iconic != 0 {
			procShowWindow.Call(uintptr(hwnd), uintptr(winSWRestore))
		}
		procShowWindow.Call(uintptr(hwnd), uintptr(winSWShow))
		procBringWindowToTop.Call(uintptr(hwnd))
		procSetForegroundWindow.Call(uintptr(hwnd))
		time.Sleep(electronSynthFocusPoll)

		fgw := foregroundHWND()
		fgPID := hwndOwningPID(fgw)
		if fgPID == myPID || fgPID == 0 || fgPID != pid {
			continue
		}
		fgw2 := foregroundHWND()
		if hwndOwningPID(fgw2) != pid {
			continue
		}

		if needsTrayRecoverPause && !trayRecoverWaitDone {
			time.Sleep(electronSynthTrayRecoverPause)
			trayRecoverWaitDone = true
			if hwndOwningPID(foregroundHWND()) != pid {
				continue
			}
		}

		if sendAltF4SendInput() {
			sendOK = true
			log.Printf("winutil: electron synth Alt+F4 SendInput pid=%d attempt=%d", pid, attempt+1)
			break
		}
	}
	if !sendOK {
		log.Printf("winutil: electron synth no Alt+F4 pid=%d fgPID=%d self=%d", pid, hwndOwningPID(foregroundHWND()), myPID)
	}
	restoreHWNDForeground(windows.HWND(fgPrior))
	sendStaleModifierKeyUps()
	return sendOK
}

func synthElectronAltF4ForegroundOnly(pid uint32) bool {
	myPID := windows.GetCurrentProcessId()
	for attempt := 0; attempt < electronSynthFocusAttempts; attempt++ {
		fgPID := hwndOwningPID(foregroundHWND())
		if fgPID == myPID || fgPID == 0 || fgPID != pid {
			time.Sleep(electronSynthFocusPoll)
			continue
		}
		if hwndOwningPID(foregroundHWND()) != pid {
			continue
		}
		if sendAltF4SendInput() {
			log.Printf("winutil: electron Alt+F4 foreground-only pid=%d attempt=%d", pid, attempt+1)
			return true
		}
		log.Printf("winutil: electron SendInput failed pid=%d", pid)
		break
	}
	log.Printf("winutil: electron foreground-only Alt+F4 missed pid=%d fgPID=%d self=%d", pid, hwndOwningPID(foregroundHWND()), myPID)
	return false
}

func requestElectronChromiumExit(exeImage string, restoreTo windows.HWND, foregroundReady bool) {
	electronSynthExitMu.Lock()
	defer electronSynthExitMu.Unlock()

	sendStaleModifierKeyUps()

	all, err := snapshotProcesses()
	if err != nil {
		log.Printf("winutil: electron exit snapshot err=%v fallback=all", err)
		requestGracefulProcessExit(exeImage)
		return
	}
	want := normalizeExeBase(exeImage)
	roots := chromiumBrowserRootPIDs(exeImage, all)
	log.Printf("winutil: electron exit image=%s rootPIDs=%d", want, len(roots))
	for _, pid := range roots {
		if foregroundReady {
			synthElectronAltF4ForegroundOnly(pid)
		} else {
			synthElectronAltF4ForPID(pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if foregroundReady && restoreTo != 0 {
		restoreHWNDForeground(restoreTo)
		sendStaleModifierKeyUps()
	}
}
