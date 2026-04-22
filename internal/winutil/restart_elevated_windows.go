//go:build windows

package winutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var singletonReleaser func()

// RegisterSingletonReleaser registers a callback run before spawning an elevated copy
// (e.g. release the app singleton mutex so the new instance can start).
func RegisterSingletonReleaser(f func()) {
	singletonReleaser = f
}

var (
	modshell32          = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW   = modshell32.NewProc("ShellExecuteW")
)

// RestartElevated re-launches the current executable with verb "runas" (UAC), forwards extraArgs,
// then exits this process. Call RegisterSingletonReleaser first so the mutex is released.
func RestartElevated(extraArgs []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)

	if singletonReleaser != nil {
		singletonReleaser()
	}

	verb, err := windows.UTF16PtrFromString("runas")
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
			return fmt.Errorf("ShellExecuteW runas: %w (code=%d)", callErr, r)
		}
		return fmt.Errorf("ShellExecuteW runas failed (code=%d)", r)
	}

	time.Sleep(300 * time.Millisecond)
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
		b.WriteString(syscall.EscapeArg(strings.TrimSpace(a)))
	}
	return b.String()
}
