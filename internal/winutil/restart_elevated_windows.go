//go:build windows

package winutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var singletonReleaser func()

// RegisterSingletonReleaser registers a callback run after a replacement is launched,
// immediately before this process exits. Failed launches keep the singleton held.
func RegisterSingletonReleaser(f func()) {
	singletonReleaser = f
}

var (
	modshell32 = windows.NewLazySystemDLL("shell32.dll")
)

// RestartElevated re-launches the current executable with verb "runas" (UAC), forwards extraArgs,
// then exits this process. The replacement waits for our exit before taking any locks.
func RestartElevated(extraArgs []string) error {
	return restartWindows("runas", extraArgs)
}

func restartWindows(verbName string, extraArgs []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)

	verb, err := windows.UTF16PtrFromString(verbName)
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(self)
	if err != nil {
		return err
	}
	params := joinArgsUTF16(restartArgs(extraArgs, os.Getpid()))
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
		if callErr == syscall.Errno(errorCancelled) {
			return ErrElevationDeclined
		}
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("ShellExecuteExW %s: %w", verbName, callErr)
		}
		return fmt.Errorf("ShellExecuteExW %s failed", verbName)
	}
	if info.hProcess == 0 {
		return fmt.Errorf("ShellExecuteExW %s returned no process handle", verbName)
	}
	_ = windows.CloseHandle(info.hProcess)
	// Do not wait for the child to acquire our singleton: Wails holds a second
	// singleton until process exit, so the child must wait for us instead.
	if singletonReleaser != nil {
		singletonReleaser()
	}
	os.Exit(0)
	return nil
}

func joinArgsUTF16(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(syscall.EscapeArg(a))
	}
	return b.String()
}
