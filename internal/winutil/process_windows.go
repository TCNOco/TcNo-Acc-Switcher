//go:build windows

package winutil

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// electronExitMaxWait caps how long we wait after gentle signals before /F taskkill.
// Too short reliably truncates Electron/LevelDB flush (Discord observed ~26s+ in practice).
const electronExitMaxWait = 35 * time.Second

const (
	electronPollInterval = 35 * time.Millisecond
)

const (
	winWMClose      = 0x0010
	winWMSysCommand = 0x0112
	winSCClose      = 0xF060
	winGWOwner      = 4
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

type procLite struct {
	PID       uint32
	ParentPID uint32
	ExeBase   string
}

var enumTopLevelMu sync.Mutex

var enumTopLevelCb struct {
	sync.Once
	ptr uintptr
}

var enumTopLevelState struct {
	pid        uint32
	chromeOnly bool
	out        *[]windows.HWND
}

// WaitForegroundForExe polls until GetForegroundWindow's owning process image matches exeImage.
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

// StartAsDesktopUser is exported for callers that always want non-inherited launch.
func StartAsDesktopUser(exe string, args []string, opts StartOpts) error {
	opts.AsDesktopUser = true
	return Start(exe, args, opts)
}
