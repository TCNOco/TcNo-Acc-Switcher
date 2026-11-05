//go:build windows

package winutil

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"TcNo-Acc-Switcher/internal/crashlog"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const servicePrefix = "SERVICE:"

const (
	gracefulExitMaxWait         = 12 * time.Second
	gracefulCombinedExitMaxWait = 5 * time.Second
	forcedExitMaxWait           = 5 * time.Second
	// gracefulQuitSettle lets a tray app re-create the hidden root window it hides on
	// first close, so the second WM_CLOSE pass has something to hit.
	gracefulQuitSettle = 200 * time.Millisecond
)

// KillByName terminates processes by image name (e.g. "steam.exe") or stops Windows services
// when the name is prefixed with SERVICE:.
// beforeElectronSynth, when non-nil, runs before Electron Alt+F4 (e.g. launch platform + wait for foreground).
func KillByName(names []string, method ClosingMethod, beforeElectronSynth func() error) error {
	return KillByNameWithOpts(names, method, KillOpts{BeforeElectronSynth: beforeElectronSynth})
}

// KillByNameWithOpts is KillByName with a platform-supplied native quit command.
func KillByNameWithOpts(names []string, method ClosingMethod, opts KillOpts) error {
	if len(names) == 0 {
		return nil
	}
	m := method
	if m == "" {
		m = ClosingCombined
	}
	log.Printf("winutil: kill begin method=%s targets=%d native=%t", m, len(names), opts.NativeQuit != nil)

	// One quit command for the whole platform, before the per-image goroutines fan out,
	// so every image is already draining while we wait on it.
	nativeQuitSent := false
	if opts.NativeQuit != nil && (m == ClosingCombined || m == ClosingClose) {
		if err := opts.NativeQuit(); err != nil {
			log.Printf("winutil: native quit failed err=%v; falling back to window messages", err)
		} else {
			nativeQuitSent = true
		}
	}

	var wg sync.WaitGroup
	failures := make(chan error, len(names))
	for _, name := range names {
		raw := strings.TrimSpace(name)
		if raw == "" {
			continue
		}
		wg.Add(1)
		go func(raw string) {
			defer crashlog.Capture()
			defer wg.Done()
			if err := stopWindowsTarget(raw, m, opts, nativeQuitSent, len(names)); err != nil {
				failures <- err
			}
		}(raw)
	}
	wg.Wait()
	close(failures)
	var errs []error
	for err := range failures {
		errs = append(errs, err)
	}
	if err := confirmStoppedProcessImages(names, SnapshotRunningExeBasenames); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	log.Printf("winutil: kill completed method=%s", m)
	return nil
}

func stopWindowsTarget(raw string, method ClosingMethod, opts KillOpts, nativeQuitSent bool, targetCount int) error {
	if strings.HasPrefix(strings.ToUpper(raw), servicePrefix) {
		name := strings.TrimSpace(raw[len(servicePrefix):])
		if err := stopWindowsService(name); err != nil {
			return fmt.Errorf("stop service %s: %w", name, err)
		}
		return nil
	}
	base := normalizeExeBase(raw)
	running, err := processExistsByImageName(base)
	if err != nil {
		return fmt.Errorf("inspect process %s: %w", base, err)
	}
	if !running {
		return nil
	}
	log.Printf("winutil: stopping process=%s method=%s", base, method)
	switch method {
	case ClosingTaskKill:
		return forceKillIfStillRunning(base)
	case ClosingElectron:
		var prior windows.HWND
		if opts.BeforeElectronSynth != nil {
			prior = foregroundHWND()
			if err := opts.BeforeElectronSynth(); err != nil {
				return fmt.Errorf("prepare process %s for closing: %w", base, err)
			}
			requestElectronChromiumExit(base, prior, true)
		} else {
			requestElectronChromiumExit(base, 0, false)
		}
		_ = taskKillIM(base, false)
		if err := waitForElectronImageExit(base, electronExitMaxWait, targetCount); err != nil && !errors.Is(err, errProcessExitTimeout) {
			return err
		}
	default:
		maxWait := gracefulCombinedExitMaxWait
		if method == ClosingClose {
			maxWait = gracefulExitMaxWait
		}
		delivered, join := requestGracefulProcessExit(base, nativeQuitSent)
		var waitErr error
		if delivered {
			waitErr = waitForImageExit(base, maxWait, 100*time.Millisecond, targetCount)
		}
		join()
		if waitErr != nil && !errors.Is(waitErr, errProcessExitTimeout) {
			return waitErr
		}
	}
	return forceKillIfStillRunning(base)
}

func forceKillIfStillRunning(exeImage string) error {
	return forceKillAndConfirm(exeImage, processExistsByImageName, func(name string) error {
		return taskKillIM(name, true)
	}, func(name string) error {
		return waitForImageExit(name, forcedExitMaxWait, 100*time.Millisecond, 1)
	})
}

func forceKillAndConfirm(exeImage string, running func(string) (bool, error), kill, wait func(string) error) error {
	alive, err := running(exeImage)
	if err != nil {
		return fmt.Errorf("inspect process %s: %w", exeImage, err)
	}
	if !alive {
		return nil
	}
	if err := kill(exeImage); err != nil {
		return fmt.Errorf("terminate process %s: %w", exeImage, err)
	}
	if err := wait(exeImage); err != nil {
		return fmt.Errorf("confirm process %s exited: %w", exeImage, err)
	}
	return nil
}

func confirmStoppedProcessImages(names []string, snapshot func() (map[string]struct{}, error)) error {
	var images []string
	for _, name := range names {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(name)), servicePrefix) {
			continue
		}
		if base := normalizeExeBase(name); base != "" {
			images = append(images, base)
		}
	}
	if len(images) == 0 {
		return nil
	}
	running, err := snapshot()
	if err != nil {
		return fmt.Errorf("confirm platform processes exited: %w", err)
	}
	var errs []error
	for _, base := range images {
		if _, alive := running[strings.ToLower(base)]; alive {
			errs = append(errs, fmt.Errorf("process %s is still running", base))
		}
	}
	return errors.Join(errs...)
}

// requestGracefulProcessExit closes every top-level window for matching PIDs (visible + hidden),
// then non-force taskkill. Electron tray apps often hide the real browser root HWND after the UI closes.
//
// The non-force taskkill runs concurrently with the caller's wait rather than before it: it only
// re-sends WM_CLOSE to windows this function already messaged, so its ~350ms belongs off the
// critical path. When a native quit command was already sent, both are skipped - asking Steam to
// minimise to tray while it is shutting down is at best useless.
//
// delivered reports whether the graceful signal reached anything that could act on it. A target
// with no top-level windows - a background service such as EABackgroundService.exe - cannot
// receive WM_CLOSE, so callers skip the wait and go straight to the force kill.
//
// The returned join must be called before the caller finishes with exeImage. A taskkill still in
// flight after KillByName returns would land on the process the switch has just relaunched.
func requestGracefulProcessExit(exeImage string, nativeQuitSent bool) (delivered bool, join func()) {
	if nativeQuitSent {
		// The platform's own quit command is the signal; there is something to wait for.
		return true, func() {}
	}
	posted := postWMCloseToMatchingProcesses(exeImage)
	if posted == 0 {
		log.Printf("winutil: image=%s has no top-level windows; skipping graceful wait", exeImage)
		return false, func() {}
	}
	done := make(chan struct{})
	go func() {
		defer crashlog.Capture()
		defer close(done)
		_ = taskKillIM(exeImage, false)
	}()
	return true, func() { <-done }
}

// postWMCloseToMatchingProcesses asks every top-level HWND owned by a matching PID to quit,
// including hidden hosts. Electron tray-only builds can keep invisible Chrome_WidgetWin_* roots;
// missing those leaves the process running, hence the second pass.
//
// The settle pause is paid once for the whole batch rather than once per PID. Steam runs 8-9
// processes during a switch, so the per-PID form cost 1.6-1.8s of pure sleep before anything
// had been asked to close.
//
// It returns how many top-level windows were reached, so the caller can tell a windowless
// target from one that was asked to close.
func postWMCloseToMatchingProcesses(exeImage string) (posted int) {
	pids, err := allPIDsForImageName(exeImage)
	if err != nil {
		log.Printf("winutil: list pids image=%s err=%v", exeImage, err)
		return 0
	}
	if len(pids) == 0 {
		return 0
	}
	for _, pid := range pids {
		posted += postGracefulQuitPass(pid)
	}
	time.Sleep(gracefulQuitSettle)
	second := 0
	for _, pid := range pids {
		second += postGracefulQuitPass(pid)
	}
	// A tray app can hide its root between passes, so either pass finding a window counts.
	if second > posted {
		posted = second
	}
	return posted
}

var gracefulQuitCb uintptr

// gracefulQuitMu serialises the enumeration so the counter below belongs to one pass.
// KillByName fans out a goroutine per image, so passes can otherwise overlap.
var (
	gracefulQuitMu    sync.Mutex
	gracefulQuitCount int
)

func init() {
	gracefulQuitCb = syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		targetPID := uint32(lParam)
		var windowPID uint32
		r0, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if r0 == 0 {
			return 1
		}
		if windowPID != targetPID {
			return 1
		}
		owner, _, _ := procGetWindow.Call(hwnd, uintptr(winGWOwner))
		if owner != 0 {
			return 1
		}
		gracefulQuitCount++
		procPostMessageW.Call(hwnd, uintptr(winWMSysCommand), uintptr(winSCClose), 0)
		procPostMessageW.Call(hwnd, uintptr(winWMClose), 0, 0)
		return 1
	})
}

// postGracefulQuitPass returns how many top-level windows it asked to close.
func postGracefulQuitPass(pid uint32) int {
	if err := procEnumWindows.Find(); err != nil {
		return 0
	}
	gracefulQuitMu.Lock()
	defer gracefulQuitMu.Unlock()
	gracefulQuitCount = 0
	_, _, _ = procEnumWindows.Call(gracefulQuitCb, uintptr(pid))
	return gracefulQuitCount
}

func syncSendCloseToHWNDs(hwnds []windows.HWND) {
	for _, h := range hwnds {
		hw := uintptr(h)
		procSendMessageW.Call(hw, uintptr(winWMSysCommand), uintptr(winSCClose), 0)
		procSendMessageW.Call(hw, uintptr(winWMClose), 0, 0)
	}
}

func stopWindowsService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		return err
	}
	defer s.Close()
	status, err := s.Query()
	if err != nil || status.State == svc.Stopped {
		return err
	}
	_, controlErr := s.Control(svc.Stop)
	if controlErr != nil && !errors.Is(controlErr, windows.ERROR_SERVICE_NOT_ACTIVE) {
		if err := forceKillIfStillRunning(name + ".exe"); err != nil {
			return errors.Join(controlErr, err)
		}
	}
	deadline := time.Now().Add(gracefulExitMaxWait)
	for {
		status, err := s.Query()
		if err != nil || status.State == svc.Stopped {
			return err
		}
		if !time.Now().Before(deadline) {
			return errors.Join(controlErr, fmt.Errorf("%w: service %s", errProcessExitTimeout, name))
		}
		time.Sleep(100 * time.Millisecond)
	}
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
