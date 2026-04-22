//go:build windows

package winutil

import (
	"syscall"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole  = kernel32.NewProc("AttachConsole")
	procAllocConsole   = kernel32.NewProc("AllocConsole")
	procFreeConsole    = kernel32.NewProc("FreeConsole")
)

// AttachParentConsole attaches to the parent process console (CLI from cmd).
func AttachParentConsole() {
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	_, _, _ = procAttachConsole.Call(attachParentProcess)
}

// AllocConsole allocates a new console if none exists (optional for headless).
func AllocConsole() error {
	r, _, err := procAllocConsole.Call()
	if r == 0 {
		return err
	}
	return nil
}

// FreeConsole frees the console for this process.
func FreeConsole() error {
	r, _, err := procFreeConsole.Call()
	if r == 0 {
		return err
	}
	return nil
}
