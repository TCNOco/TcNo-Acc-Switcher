//go:build windows

package winutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procShellExecuteExW = modshell32.NewProc("ShellExecuteExW")

// ErrElevationDeclined is returned when the user dismisses the UAC prompt.
var ErrElevationDeclined = errors.New("elevation declined")

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	swShowNormal          = 1
	errorCancelled        = 1223
)

// shellExecuteInfoW mirrors SHELLEXECUTEINFOW. x/sys/windows exposes
// ShellExecuteW but not the Ex form, and only Ex hands back a process handle to
// wait on.
type shellExecuteInfoW struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.HWND
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIcon        windows.Handle
	hProcess     windows.Handle
}

// RunSelfElevatedAndWait starts another copy of this executable with the "runas"
// verb, waits up to timeout for it to exit and returns its exit code. The
// current process keeps running - unlike [RestartElevated], which hands over and
// exits, so the singleton mutex is deliberately not released here. The child
// must therefore be a mode that does not take the singleton.
//
// Returns [ErrElevationDeclined] when the user dismisses the UAC prompt.
func RunSelfElevatedAndWait(args []string, timeout time.Duration) (int, error) {
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	self = filepath.Clean(self)

	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return 0, err
	}
	file, err := windows.UTF16PtrFromString(self)
	if err != nil {
		return 0, err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(self))
	if err != nil {
		return 0, err
	}
	var paramsPtr *uint16
	if params := joinArgsUTF16(args); params != "" {
		p, perr := windows.UTF16PtrFromString(params)
		if perr != nil {
			return 0, perr
		}
		paramsPtr = p
	}

	info := shellExecuteInfoW{
		fMask:        seeMaskNoCloseProcess | seeMaskNoAsync,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: paramsPtr,
		lpDirectory:  dir,
		nShow:        swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))

	r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorCancelled {
			return 0, ErrElevationDeclined
		}
		if callErr != nil && callErr != syscall.Errno(0) {
			return 0, fmt.Errorf("ShellExecuteExW runas: %w", callErr)
		}
		return 0, errors.New("ShellExecuteExW runas failed")
	}
	if info.hProcess == 0 {
		return 0, errors.New("ShellExecuteExW returned no process handle")
	}
	defer windows.CloseHandle(info.hProcess)

	wait := uint32(windows.INFINITE)
	if timeout > 0 {
		wait = uint32(timeout / time.Millisecond)
	}
	switch ev, werr := windows.WaitForSingleObject(info.hProcess, wait); {
	case werr != nil:
		return 0, werr
	case ev == uint32(windows.WAIT_TIMEOUT):
		return 0, fmt.Errorf("elevated process did not exit within %s", timeout)
	}

	var code uint32
	if err := windows.GetExitCodeProcess(info.hProcess, &code); err != nil {
		return 0, err
	}
	return int(code), nil
}
