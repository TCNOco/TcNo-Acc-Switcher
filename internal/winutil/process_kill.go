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

	"TcNo-Acc-Switcher/internal/crashlog"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const servicePrefix = "SERVICE:"

const (
	gracefulExitMaxWait         = 12 * time.Second
	gracefulCombinedExitMaxWait = 5 * time.Second
	// gracefulQuitSettle lets a tray app re-create the hidden root window it hides on
	// first close, so the second WM_CLOSE pass has something to hit.
	gracefulQuitSettle = 200 * time.Millisecond
)

// KillOpts tunes how KillByName ends a platform.
type KillOpts struct {
	// NativeQuit, when non-nil, issues the platform's own quit command once before any
	// window messages are sent, and replaces the WM_CLOSE broadcast as the graceful signal.
	//
	// Steam needs this: it treats WM_CLOSE on its main window as "minimise to tray", so the
	// WM_CLOSE path can never make it exit and the graceful window burns its full deadline
	// on every single switch. "steam.exe -shutdown" drains the whole tree in ~1.6-2.5s.
	NativeQuit func() error
	// BeforeElectronSynth runs before Electron Alt+F4 (e.g. launch platform + wait for foreground).
	BeforeElectronSynth func() error
}

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
	for _, name := range names {
		raw := strings.TrimSpace(name)
		if raw == "" {
			continue
		}
		wg.Add(1)
		go func(raw string) {
			defer crashlog.Capture()
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
			// An image that is not running costs two ~350ms taskkill spawns for nothing.
			// GameOverlayUI.exe is absent on most Steam switches.
			if running, err := processExistsByImageName(base); err == nil && !running {
				log.Printf("winutil: skipping process=%s (not running)", base)
				return
			}
			log.Printf("winutil: stopping process=%s method=%s", base, m)
			switch m {
			case ClosingTaskKill:
				_ = taskKillIM(base, true)
			case ClosingElectron:
				var prior windows.HWND
				if opts.BeforeElectronSynth != nil {
					prior = foregroundHWND()
					if err := opts.BeforeElectronSynth(); err != nil {
						log.Printf("winutil: electron prepare err=%v", err)
					}
					requestElectronChromiumExit(base, prior, true)
				} else {
					requestElectronChromiumExit(base, 0, false)
				}
				_ = taskKillIM(base, false)
				waitForElectronImageExit(base, electronExitMaxWait, len(names))
				forceKillIfStillRunning(base)
			case ClosingClose:
				join := requestGracefulProcessExit(base, nativeQuitSent)
				waitForImageExit(base, gracefulExitMaxWait, 100*time.Millisecond, len(names))
				join()
				forceKillIfStillRunning(base)
			default: // Combined
				join := requestGracefulProcessExit(base, nativeQuitSent)
				waitForImageExit(base, gracefulCombinedExitMaxWait, 100*time.Millisecond, len(names))
				join()
				forceKillIfStillRunning(base)
			}
			log.Printf("winutil: stop process done process=%s", base)
		}(raw)
	}
	wg.Wait()
	log.Printf("winutil: kill completed method=%s", m)
	return nil
}

// forceKillIfStillRunning skips the ~350ms taskkill spawn when the image already exited,
// which is the normal case once the graceful signal actually works.
func forceKillIfStillRunning(exeImage string) {
	if running, err := processExistsByImageName(exeImage); err == nil && !running {
		return
	}
	_ = taskKillIM(exeImage, true)
}

// requestGracefulProcessExit closes every top-level window for matching PIDs (visible + hidden),
// then non-force taskkill. Electron tray apps often hide the real browser root HWND after the UI closes.
//
// The non-force taskkill runs concurrently with the caller's wait rather than before it: it only
// re-sends WM_CLOSE to windows this function already messaged, so its ~350ms belongs off the
// critical path. When a native quit command was already sent, both are skipped - asking Steam to
// minimise to tray while it is shutting down is at best useless.
//
// The returned join must be called before the caller finishes with exeImage. A taskkill still in
// flight after KillByName returns would land on the process the switch has just relaunched.
func requestGracefulProcessExit(exeImage string, nativeQuitSent bool) (join func()) {
	if nativeQuitSent {
		return func() {}
	}
	postWMCloseToMatchingProcesses(exeImage)
	done := make(chan struct{})
	go func() {
		defer crashlog.Capture()
		defer close(done)
		_ = taskKillIM(exeImage, false)
	}()
	return func() { <-done }
}

// postWMCloseToMatchingProcesses asks every top-level HWND owned by a matching PID to quit,
// including hidden hosts. Electron tray-only builds can keep invisible Chrome_WidgetWin_* roots;
// missing those leaves the process running, hence the second pass.
//
// The settle pause is paid once for the whole batch rather than once per PID. Steam runs 8-9
// processes during a switch, so the per-PID form cost 1.6-1.8s of pure sleep before anything
// had been asked to close.
func postWMCloseToMatchingProcesses(exeImage string) {
	pids, err := allPIDsForImageName(exeImage)
	if err != nil {
		log.Printf("winutil: list pids image=%s err=%v", exeImage, err)
		return
	}
	if len(pids) == 0 {
		return
	}
	for _, pid := range pids {
		postGracefulQuitPass(pid)
	}
	time.Sleep(gracefulQuitSettle)
	for _, pid := range pids {
		postGracefulQuitPass(pid)
	}
}

var gracefulQuitCb uintptr

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
		procPostMessageW.Call(hwnd, uintptr(winWMSysCommand), uintptr(winSCClose), 0)
		procPostMessageW.Call(hwnd, uintptr(winWMClose), 0, 0)
		return 1
	})
}

func postGracefulQuitPass(pid uint32) {
	if err := procEnumWindows.Find(); err != nil {
		return
	}
	_, _, _ = procEnumWindows.Call(gracefulQuitCb, uintptr(pid))
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
