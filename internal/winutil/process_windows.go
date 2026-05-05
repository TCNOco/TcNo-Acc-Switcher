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

// gracefulExitMaxWait is how long we wait after asking a process to shut down gracefully
// before escalating to forced taskkill. Electron apps such as Discord need time to flush LevelDB.
const gracefulExitMaxWait = 10 * time.Second

const (
	WM_CLOSE           = 0x0010
	WM_SYSCOMMAND      = 0x0112
	SC_CLOSE           = 0xF060
	WM_ENDSESSION      = 0x0016
	WM_QUERYENDSESSION = 0x0011
	winGWOwner         = 4
)

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindow                = modUser32.NewProc("GetWindow")
	procPostMessageW             = modUser32.NewProc("PostMessageW")
	procSendMessageW             = modUser32.NewProc("SendMessageW")
)

// KillByName terminates processes by image name (e.g. "steam.exe") or stops Windows services
// when the name is prefixed with SERVICE:.
func KillByName(names []string, method ClosingMethod) error {
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
	want = strings.TrimSpace(want)
	if want == "" {
		return nil, nil
	}
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
	var out []uint32
	for {
		exe := utf16FixedToString(pe.ExeFile[:])
		if strings.EqualFold(exe, want) {
			out = append(out, pe.ProcessID)
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return out, nil
			}
			return out, err
		}
	}
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

// postGracefulQuitForPID asks every top-level HWND owned by pid to quit, including hidden hosts
// Electron keeps (e.g. tray-only mode) relies on invisible Chrome_WidgetWin_* roots; skipping those
// leaves DiscordPTB.exe running after the visible window disappears.
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
		// Match the title-bar X path when the window has a system menu; harmless if ignored.
		procPostMessageW.Call(hwnd, uintptr(WM_SYSCOMMAND), uintptr(SC_CLOSE), 0)
		procPostMessageW.Call(hwnd, uintptr(WM_CLOSE), 0, 0)
		procPostMessageW.Call(hwnd, uintptr(WM_QUERYENDSESSION), 1, 0)
		procSendMessageW.Call(hwnd, uintptr(WM_ENDSESSION), 1, 0)
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

func processExistsByImageName(want string) (bool, error) {
	want = strings.TrimSpace(want)
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
