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
	"TcNo-Acc-Switcher/internal/crashlog"
)

const servicePrefix = "SERVICE:"

const (
	gracefulExitMaxWait         = 12 * time.Second
	gracefulCombinedExitMaxWait = 5 * time.Second
)

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
				waitForElectronImageExit(base, electronExitMaxWait, len(names))
				_ = taskKillIM(base, true)
			case ClosingClose:
				requestGracefulProcessExit(base)
				waitForImageExit(base, gracefulExitMaxWait, 100*time.Millisecond, len(names))
				_ = taskKillIM(base, true)
			default: // Combined
				requestGracefulProcessExit(base)
				waitForImageExit(base, gracefulCombinedExitMaxWait, 100*time.Millisecond, len(names))
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
	time.Sleep(200 * time.Millisecond)
	postGracefulQuitPass(pid)
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
