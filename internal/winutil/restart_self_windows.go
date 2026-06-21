//go:build windows

package winutil

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RestartSelf re-launches the current executable with extraArgs, then exits this process.
func RestartSelf(extraArgs []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)

	if singletonReleaser != nil {
		singletonReleaser()
	}

	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(self)
	if err != nil {
		return err
	}
	params := joinArgsUTF16(extraArgs)
	var paramsPtr *uint16
	if params != "" {
		p, err := windows.UTF16PtrFromString(params)
		if err != nil {
			return err
		}
		paramsPtr = p
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(self))
	if err != nil {
		return err
	}

	const swShow = 5
	r, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(paramsPtr)),
		uintptr(unsafe.Pointer(dir)),
		swShow,
	)
	if r <= 32 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("ShellExecuteW: %w (code=%d)", callErr, r)
		}
		return fmt.Errorf("ShellExecuteW failed (code=%d)", r)
	}

	name, _ := windows.UTF16PtrFromString(singletonMutexName)
	for i := 0; i < 30; i++ {
		h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
		if err == nil {
			windows.CloseHandle(h)
			os.Exit(0)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("restart failed: child process did not start within 3s")
}
