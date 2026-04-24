//go:build windows

package winutil

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	seIncreaseQuotaPrivilege = "SeIncreaseQuotaPrivilege"
	sePrivilegeEnabled       = uint32(0x00000002)
	tokenDupRights           = uint32(0x0002) // TOKEN_DUPLICATE
	duplicateTokenRights     = uint32(0x18B) // minimal set for CreateProcessWithTokenW (matches C#)
)

var (
	modadvapi32                 = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithTokenW = modadvapi32.NewProc("CreateProcessWithTokenW")
)

// runAsDesktopUser starts exe with args under the interactive shell user's token (not inherited admin).
// Caller must ensure the current process is elevated; SeIncreaseQuotaPrivilege must be usable.
func runAsDesktopUser(exe string, args []string, workingDir string, hideWindow bool) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}

	// 1) Enable SeIncreaseQuotaPrivilege on current process token.
	hProcessToken := windows.Token(0)
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &hProcessToken)
	if err != nil {
		return fmt.Errorf("OpenProcessToken(self): %w", err)
	}
	defer hProcessToken.Close()

	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(seIncreaseQuotaPrivilege), &luid); err != nil {
		return fmt.Errorf("LookupPrivilegeValue: %w", err)
	}
	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{
			{Luid: luid, Attributes: sePrivilegeEnabled},
		},
	}
	if err := windows.AdjustTokenPrivileges(hProcessToken, false, &tp, 0, nil, nil); err != nil {
		return fmt.Errorf("AdjustTokenPrivileges: %w", err)
	}

	// 2) Shell window → shell PID.
	hwnd := windows.GetShellWindow()
	if hwnd == 0 {
		return fmt.Errorf("GetShellWindow returned 0")
	}
	var shellPID uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &shellPID); err != nil {
		return fmt.Errorf("GetWindowThreadProcessId: %w", err)
	}
	if shellPID == 0 {
		return fmt.Errorf("shell PID is 0")
	}

	hShellProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, shellPID)
	if err != nil {
		return fmt.Errorf("OpenProcess(shell): %w", err)
	}
	defer windows.CloseHandle(hShellProcess)

	var hShellToken windows.Token
	if err := windows.OpenProcessToken(hShellProcess, tokenDupRights, &hShellToken); err != nil {
		return fmt.Errorf("OpenProcessToken(shell): %w", err)
	}
	defer hShellToken.Close()

	var hPrimary windows.Token
	if err := windows.DuplicateTokenEx(
		hShellToken,
		duplicateTokenRights,
		nil,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&hPrimary,
	); err != nil {
		return fmt.Errorf("DuplicateTokenEx: %w", err)
	}
	defer hPrimary.Close()

	appUTF16, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	cmdLine := buildCommandLineForCreateProcess(args)
	var cmdUTF16 *uint16
	if strings.TrimSpace(cmdLine) != "" {
		cmdUTF16, err = windows.UTF16PtrFromString(cmdLine)
		if err != nil {
			return err
		}
	}

	wd := strings.TrimSpace(workingDir)
	if wd == "" {
		wd = filepath.Dir(exe)
	}
	wdUTF16, err := windows.UTF16PtrFromString(wd)
	if err != nil {
		return err
	}

	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	if hideWindow {
		si.Flags |= windows.STARTF_USESHOWWINDOW
		si.ShowWindow = windows.SW_HIDE
	}
	var pi windows.ProcessInformation

	r1, _, callErr := procCreateProcessWithTokenW.Call(
		uintptr(hPrimary),
		0,
		uintptr(unsafe.Pointer(appUTF16)),
		uintptr(unsafe.Pointer(cmdUTF16)),
		0,
		0,
		uintptr(unsafe.Pointer(wdUTF16)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r1 == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("CreateProcessWithTokenW: %w", callErr)
		}
		return fmt.Errorf("CreateProcessWithTokenW failed")
	}
	if pi.Process != 0 {
		_ = windows.CloseHandle(pi.Process)
	}
	if pi.Thread != 0 {
		_ = windows.CloseHandle(pi.Thread)
	}
	return nil
}

func buildCommandLineForCreateProcess(args []string) string {
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
	// C# RunAsDesktopUser prepends a leading space before args when passed to CreateProcessWithTokenW.
	return " " + b.String()
}

// tryRunAsDesktopUser logs and returns false on failure (for fallback).
func tryRunAsDesktopUser(exe string, args []string, workingDir string, hideWindow bool) bool {
	if err := runAsDesktopUser(exe, args, workingDir, hideWindow); err != nil {
		log.Printf("winutil: RunAsDesktopUser failed for %s: %v", exe, err)
		return false
	}
	return true
}
