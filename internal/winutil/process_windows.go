//go:build windows

package winutil

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const servicePrefix = "SERVICE:"

// gracefulExitMaxWait is how long we wait after non-force shutdown before escalating to /F taskkill.
const gracefulExitMaxWait = 12 * time.Second

// electronExitMaxWait caps how long we wait after gentle signals before /F taskkill.
// Too short reliably truncates Electron/LevelDB flush (Discord observed ~26s+ in practice).
const electronExitMaxWait = 35 * time.Second

const (
	// electronPollInterval only affects how soon we detect the process is gone; does not rush /F.
	electronPollInterval = 35 * time.Millisecond

	// Synth Alt+F4: SetForeground often fails on first try; never SendInput unless foreground owns target PID.
	electronSynthFocusAttempts = 12
	electronSynthFocusPoll     = 50 * time.Millisecond

	// Tray-only / minimized Chromium: after SW_RESTORE + foreground, wait before Alt+F4 so WebContents can activate.
	electronSynthTrayRecoverPause = 950 * time.Millisecond
)

const chromeWidgetWinClass = "Chrome_WidgetWin_1"

const (
	winWMClose        = 0x0010
	winWMSysCommand   = 0x0112
	winSCClose        = 0xF060
	winGWOwner        = 4
	winKEYEVENTFKeyUp = 0x0002
	winVKMENU         = 0x12 // VK_MENU — left or right Alt
	winVKLMENU        = 0xA4
	winVKRMENU        = 0xA5
	winVKF4           = 0x73
	winINPUTKeyboard  = 1
	winSWRestore      = 9
	winSWShow         = 5
)

var (
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId       = modKernel32.NewProc("GetCurrentThreadId")
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindow                = modUser32.NewProc("GetWindow")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procPostMessageW             = modUser32.NewProc("PostMessageW")
	procSendMessageW             = modUser32.NewProc("SendMessageW")
	procGetClassNameW            = modUser32.NewProc("GetClassNameW")
	procSendInput                = modUser32.NewProc("SendInput")
	procSetForegroundWindow      = modUser32.NewProc("SetForegroundWindow")
	procAttachThreadInput        = modUser32.NewProc("AttachThreadInput")
	procBringWindowToTop         = modUser32.NewProc("BringWindowToTop")
	procShowWindow               = modUser32.NewProc("ShowWindow")
	procIsIconic                 = modUser32.NewProc("IsIconic")
	procIsWindowVisible          = modUser32.NewProc("IsWindowVisible")
)

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

type procLite struct {
	PID       uint32
	ParentPID uint32
	ExeBase   string
}

func normalizeExeBase(s string) string {
	s = strings.TrimSpace(filepath.Base(s))
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(s), ".exe") {
		s += ".exe"
	}
	return s
}

func snapshotProcesses() ([]procLite, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		if err == windows.ERROR_NO_MORE_FILES {
			return nil, nil
		}
		return nil, err
	}
	var out []procLite
	for {
		exe := utf16FixedToString(pe.ExeFile[:])
		out = append(out, procLite{
			PID:       pe.ProcessID,
			ParentPID: pe.ParentProcessID,
			ExeBase:   exe,
		})
		if err := windows.Process32Next(snap, &pe); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return out, nil
			}
			return out, err
		}
	}
}

func chromiumBrowserRootPIDs(wantExe string, all []procLite) []uint32 {
	want := normalizeExeBase(wantExe)
	if want == "" || len(all) == 0 {
		return nil
	}
	sameImage := make(map[uint32]bool)
	for _, p := range all {
		if strings.EqualFold(p.ExeBase, want) {
			sameImage[p.PID] = true
		}
	}
	var roots []uint32
	for _, p := range all {
		if !strings.EqualFold(p.ExeBase, want) {
			continue
		}
		if !sameImage[p.ParentPID] {
			roots = append(roots, p.PID)
		}
	}
	if len(roots) == 0 {
		for _, p := range all {
			if strings.EqualFold(p.ExeBase, want) {
				roots = append(roots, p.PID)
			}
		}
	}
	return roots
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

// enumTopLevelMu serializes EnumWindows callbacks that append into caller-owned slices (no LPARAM userdata).
var enumTopLevelMu sync.Mutex

// electronSynthExitMu prevents overlapping AttachThreadInput / synth SendInput sequences.
var electronSynthExitMu sync.Mutex

var enumTopLevelCb struct {
	sync.Once
	ptr uintptr
}

var enumTopLevelState struct {
	pid        uint32
	chromeOnly bool
	out        *[]windows.HWND
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

func syncSendCloseToHWNDs(hwnds []windows.HWND) {
	for _, h := range hwnds {
		hw := uintptr(h)
		procSendMessageW.Call(hw, uintptr(winWMSysCommand), uintptr(winSCClose), 0)
		procSendMessageW.Call(hw, uintptr(winWMClose), 0, 0)
	}
}

func windowThreadID(hwnd windows.HWND) uint32 {
	t, _, _ := procGetWindowThreadProcessId.Call(uintptr(hwnd), 0)
	return uint32(t)
}

// hwndOwningPID returns the process id that owns hwnd (foreground checks use this; never SendInput Alt+F4 unless this matches target).
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

func exeBaseForPIDInSnapshot(all []procLite, pid uint32) string {
	for _, p := range all {
		if p.PID == pid {
			return p.ExeBase
		}
	}
	return ""
}

// WaitForegroundForExe polls until GetForegroundWindow’s owning process image matches exeImage.
func WaitForegroundForExe(exeImage string, maxWait time.Duration) bool {
	want := normalizeExeBase(exeImage)
	if want == "" {
		return false
	}
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		fg := foregroundHWND()
		if fg == 0 {
			time.Sleep(85 * time.Millisecond)
			continue
		}
		pid := hwndOwningPID(fg)
		if pid == 0 {
			time.Sleep(85 * time.Millisecond)
			continue
		}
		all, err := snapshotProcesses()
		if err != nil {
			time.Sleep(85 * time.Millisecond)
			continue
		}
		if strings.EqualFold(normalizeExeBase(exeBaseForPIDInSnapshot(all, pid)), want) {
			return true
		}
		time.Sleep(85 * time.Millisecond)
	}
	return false
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

// attachForegroundAndTarget attaches foreground + target UI threads so SetForegroundWindow can move focus cross-process.
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

// synthHWNDElectron picks a Chromium top-level HWND to receive synthetic keyboard focus.
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

// sendStaleModifierKeyUps releases Alt/F4 in case prior synth interrupted mid-chord (next focus round needs clean state).
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

// synthElectronAltF4ForPID activates a Chromium HWND (ShowWindow/foreground) then SendInput Alt+F4 when focus cannot be assumed.
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
		// Immediate re-check — focus can race with other UI.
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

// synthElectronAltF4ForegroundOnly sends Alt+F4 only when the foreground already belongs to pid (after caller ran normal launch).
// Does not restore prior foreground — caller must do that once after all browser-root PIDs (Discord can have several roots).
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

// requestElectronChromiumExit sends synthetic Alt+F4 per browser root PID. No WM_CLOSE / SC_CLOSE.
// When foregroundReady, caller has already foregrounded the app (e.g. Launch platform); restoreTo is the HWND to refocus afterward.
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

// KillByName terminates processes by image name (e.g. "steam.exe") or stops Windows services
// when the name is prefixed with SERVICE:.
// beforeElectronSynth, when non-nil, runs before Electron Alt+F4 (e.g. launch platform + wait for foreground).
func KillByName(names []string, method ClosingMethod, beforeElectronSynth func() error) error {
	if len(names) == 0 {
		return nil
	}
	m := method
	if m == "" {
		m = ClosingCombined
	}
	log.Printf("winutil: kill begin method=%s targets=%d", m, len(names))
	var wg sync.WaitGroup
	for _, name := range names {
		raw := strings.TrimSpace(name)
		if raw == "" {
			continue
		}
		wg.Add(1)
		go func(raw string) {
			defer wg.Done()
			if strings.HasPrefix(strings.ToUpper(raw), strings.ToUpper(servicePrefix)) {
				svcName := strings.TrimSpace(raw[len(servicePrefix):])
				log.Printf("winutil: stopping service=%s", svcName)
				if err := stopWindowsService(svcName); err != nil {
					log.Printf("winutil: stop service failed service=%s err=%v; trying process kill fallback", svcName, err)
					_ = taskKillIM(svcName+".exe", true)
				}
				log.Printf("winutil: stop service done service=%s", svcName)
				return
			}
			base := filepath.Base(raw)
			if !strings.HasSuffix(strings.ToLower(base), ".exe") {
				base = raw + ".exe"
			}
			log.Printf("winutil: stopping process=%s method=%s", base, m)
			switch m {
			case ClosingTaskKill:
				_ = taskKillIM(base, true)
			case ClosingElectron:
				var prior windows.HWND
				if beforeElectronSynth != nil {
					prior = foregroundHWND()
					if err := beforeElectronSynth(); err != nil {
						log.Printf("winutil: electron prepare err=%v", err)
					}
					requestElectronChromiumExit(base, prior, true)
				} else {
					requestElectronChromiumExit(base, 0, false)
				}
				_ = taskKillIM(base, false)
				waitForElectronImageExit(base, electronExitMaxWait)
				_ = taskKillIM(base, true)
			case ClosingClose:
				requestGracefulProcessExit(base)
				waitForImageExit(base, gracefulExitMaxWait, 100*time.Millisecond)
				_ = taskKillIM(base, true)
			default: // Combined
				requestGracefulProcessExit(base)
				waitForImageExit(base, gracefulExitMaxWait, 100*time.Millisecond)
				_ = taskKillIM(base, true)
			}
			log.Printf("winutil: stop process done process=%s", base)
		}(raw)
	}
	wg.Wait()
	log.Printf("winutil: kill completed method=%s", m)
	return nil
}

// requestGracefulProcessExit closes every top-level window for matching PIDs (visible + hidden),
// then non-force taskkill. Electron tray apps often hide the real browser root HWND after the UI closes.
func requestGracefulProcessExit(exeImage string) {
	postWMCloseToMatchingProcesses(exeImage)
	_ = taskKillIM(exeImage, false)
}

func allPIDsForImageName(want string) ([]uint32, error) {
	want = normalizeExeBase(want)
	if want == "" {
		return nil, nil
	}
	all, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	var out []uint32
	for _, p := range all {
		if strings.EqualFold(p.ExeBase, want) {
			out = append(out, p.PID)
		}
	}
	return out, nil
}

func postWMCloseToMatchingProcesses(exeImage string) {
	pids, err := allPIDsForImageName(exeImage)
	if err != nil {
		log.Printf("winutil: list pids image=%s err=%v", exeImage, err)
		return
	}
	for _, pid := range pids {
		postGracefulQuitForPID(pid)
	}
}

// postGracefulQuitForPID asks every top-level HWND owned by pid to quit, including hidden hosts.
// Electron tray-only builds can keep invisible Chrome_WidgetWin_* roots; missing those leaves the process running.
func postGracefulQuitForPID(pid uint32) {
	postGracefulQuitPass(pid)
	// Second pass after Chromium/Electron tears down layered UI in stages.
	time.Sleep(200 * time.Millisecond)
	postGracefulQuitPass(pid)
}

func postGracefulQuitPass(pid uint32) {
	if err := procEnumWindows.Find(); err != nil {
		return
	}
	cb := syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		targetPID := uint32(lParam)
		var windowPID uint32
		r0, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if r0 == 0 {
			return 1 // continue enumeration
		}
		if windowPID != targetPID {
			return 1
		}
		owner, _, _ := procGetWindow.Call(hwnd, uintptr(winGWOwner))
		if owner != 0 {
			return 1
		}
		// Async posts; never send WM_QUERYENDSESSION/WM_ENDSESSION (session shutdown semantics).
		procPostMessageW.Call(hwnd, uintptr(winWMSysCommand), uintptr(winSCClose), 0)
		procPostMessageW.Call(hwnd, uintptr(winWMClose), 0, 0)
		return 1
	})
	_, _, _ = procEnumWindows.Call(cb, uintptr(pid))
}

// waitForImageExit polls until exeImage is gone or maxWait elapses.
func waitForImageExit(exeImage string, maxWait, poll time.Duration) {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		exists, err := processExistsByImageName(exeImage)
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			return
		}
		if !exists {
			return
		}
		time.Sleep(poll)
	}
}

// waitForElectronImageExit polls often so we return immediately once the image is gone; we do not
// send extra taskkill nudges mid-wait (those can interrupt a partially finished graceful shutdown).
func waitForElectronImageExit(exeImage string, maxWait time.Duration) {
	waitForImageExit(exeImage, maxWait, electronPollInterval)
}

func processExistsByImageName(want string) (bool, error) {
	want = normalizeExeBase(want)
	if want == "" {
		return false, nil
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		if err == windows.ERROR_NO_MORE_FILES {
			return false, nil
		}
		return false, err
	}
	for {
		exe := utf16FixedToString(pe.ExeFile[:])
		if strings.EqualFold(exe, want) {
			return true, nil
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return false, nil
			}
			return false, err
		}
	}
}

func utf16FixedToString(b []uint16) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return windows.UTF16ToString(b[:n])
}

func stopWindowsService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()
	_, err = s.Control(svc.Stop)
	return err
}

func taskKillIM(name string, force bool) error {
	args := []string{"/C", "taskkill"}
	if force {
		args = append(args, "/F")
	}
	args = append(args, "/T", "/IM", name)
	cmd := exec.Command("cmd.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if strings.Contains(s, "not running") || strings.Contains(s, "could not find") || strings.Contains(s, "not found") {
			return nil
		}
		return fmt.Errorf("taskkill: %w: %s", err, s)
	}
	return nil
}

// IsProcessElevated returns true if the current process is running elevated (admin).
func IsProcessElevated() bool {
	var tok windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok)
	if err != nil {
		return false
	}
	defer tok.Close()
	return tok.IsElevated()
}

// Start launches exe with args. Uses PowerShell Start-Process -Verb RunAs when opts.Admin.
func Start(exe string, args []string, opts StartOpts) error {
	if opts.AsDesktopUser && IsProcessElevated() {
		slogWin().Debug("start request", "exe", exe, "mode", "desktop-user", "args", len(args), "admin", opts.Admin, "method", opts.Method)
		return startAsDesktopUser(exe, args, opts)
	}
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
	slogWin().Debug("start request", "exe", exe, "args", len(args), "admin", opts.Admin, "method", opts.Method, "workingDir", strings.TrimSpace(opts.WorkingDir))
	if opts.Admin {
		err := startElevated(exe, args, opts)
		if err != nil {
			slogWin().Warn("start failed", "exe", exe, "err", err)
			return err
		}
		slogWin().Debug("start launched", "exe", exe, "mode", "elevated")
		return nil
	}
	cmd := exec.Command(exe, args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: opts.HideWindow}
	if err := cmd.Start(); err != nil {
		slogWin().Warn("start failed", "exe", exe, "err", err)
		return WrapIfElevationRequired(err)
	}
	slogWin().Debug("start launched", "exe", exe, "pid", cmd.Process.Pid)
	return nil
}

func startElevated(exe string, args []string, opts StartOpts) error {
	var b strings.Builder
	b.WriteString(`Start-Process -FilePath `)
	b.WriteString(fmt.Sprintf("%q", exe))
	if len(args) > 0 {
		b.WriteString(` -ArgumentList `)
		b.WriteString(psArgList(args))
	}
	if wd := strings.TrimSpace(opts.WorkingDir); wd != "" {
		b.WriteString(` -WorkingDirectory `)
		b.WriteString(fmt.Sprintf("%q", wd))
	}
	b.WriteString(` -Verb RunAs`)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", b.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start elevated: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func psArgList(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("@(")
	for i, a := range args {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("'")
		b.WriteString(strings.ReplaceAll(a, "'", "''"))
		b.WriteString("'")
	}
	b.WriteString(")")
	return b.String()
}

// startAsDesktopUser avoids inheriting admin when the switcher is elevated.
// Prefer CreateProcessWithTokenW (shell user token); fall back to cmd /c start if that fails.
func startAsDesktopUser(exe string, args []string, opts StartOpts) error {
	wd := strings.TrimSpace(opts.WorkingDir)
	if tryRunAsDesktopUser(exe, args, wd, opts.HideWindow) {
		return nil
	}
	slogWin().Debug("falling back to cmd start", "exe", exe)
	cmdline := append([]string{"/c", "start", "", exe}, args...)
	cmd := exec.Command("cmd.exe", cmdline...)
	if wd != "" {
		cmd.Dir = wd
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: opts.HideWindow}
	return WrapIfElevationRequired(cmd.Start())
}

// StartAsDesktopUser is exported for callers that always want non-inherited launch.
func StartAsDesktopUser(exe string, args []string, opts StartOpts) error {
	opts.AsDesktopUser = true
	return Start(exe, args, opts)
}
