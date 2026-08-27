//go:build windows

package winutil

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	seIncreaseQuotaPrivilege = "SeIncreaseQuotaPrivilege"
	sePrivilegeEnabled       = uint32(0x00000002)
	tokenDupRights           = uint32(0x0002) // TOKEN_DUPLICATE
	duplicateTokenRights     = uint32(0x18B)  // minimal for CreateProcessWithTokenW
)

var (
	modadvapi32                 = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithTokenW = modadvapi32.NewProc("CreateProcessWithTokenW")
)

// runAsDesktopUser runs under the logged-in user's token (not inherited admin); requires elevation + SeIncreaseQuotaPrivilege.
// The launch itself goes through [spawnWithToken], so the target is decoupled from us like any other.
func runAsDesktopUser(exe string, args []string, workingDir string, hideWindow bool) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
	token, err := desktopUserToken()
	if err != nil {
		return err
	}
	defer token.Close()
	if _, err := spawnWithToken(exe, args, workingDir, hideWindow, token); err != nil {
		return WrapIfElevationRequired(err)
	}
	return nil
}

// desktopUserToken duplicates the shell's token into a primary token so a launch drops our elevation.
func desktopUserToken() (windows.Token, error) {
	hProcessToken := windows.Token(0)
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &hProcessToken)
	if err != nil {
		return 0, fmt.Errorf("OpenProcessToken(self): %w", err)
	}
	defer hProcessToken.Close()

	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(seIncreaseQuotaPrivilege), &luid); err != nil {
		return 0, fmt.Errorf("LookupPrivilegeValue: %w", err)
	}
	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{
			{Luid: luid, Attributes: sePrivilegeEnabled},
		},
	}
	if err := windows.AdjustTokenPrivileges(hProcessToken, false, &tp, 0, nil, nil); err != nil {
		return 0, fmt.Errorf("AdjustTokenPrivileges: %w", err)
	}

	hwnd := windows.GetShellWindow()
	if hwnd == 0 {
		return 0, fmt.Errorf("GetShellWindow returned 0")
	}
	var shellPID uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &shellPID); err != nil {
		return 0, fmt.Errorf("GetWindowThreadProcessId: %w", err)
	}
	if shellPID == 0 {
		return 0, fmt.Errorf("shell PID is 0")
	}

	hShellProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, shellPID)
	if err != nil {
		return 0, fmt.Errorf("OpenProcess(shell): %w", err)
	}
	defer windows.CloseHandle(hShellProcess)

	var hShellToken windows.Token
	if err := windows.OpenProcessToken(hShellProcess, tokenDupRights, &hShellToken); err != nil {
		return 0, fmt.Errorf("OpenProcessToken(shell): %w", err)
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
		return 0, fmt.Errorf("DuplicateTokenEx: %w", err)
	}
	return hPrimary, nil
}

func createProcessWithToken(token windows.Token, app, cmdLine *uint16, flags uint32, wd *uint16, si *windows.StartupInfo, pi *windows.ProcessInformation) error {
	r1, _, callErr := procCreateProcessWithTokenW.Call(
		uintptr(token),
		0, // dwLogonFlags
		uintptr(unsafe.Pointer(app)),
		uintptr(unsafe.Pointer(cmdLine)),
		uintptr(flags),
		0, // lpEnvironment: inherit ours
		uintptr(unsafe.Pointer(wd)),
		uintptr(unsafe.Pointer(si)),
		uintptr(unsafe.Pointer(pi)),
	)
	if r1 == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("CreateProcessWithTokenW: %w", callErr)
		}
		return fmt.Errorf("CreateProcessWithTokenW failed")
	}
	return nil
}

func tryRunAsDesktopUser(exe string, args []string, workingDir string, hideWindow bool) bool {
	if err := runAsDesktopUser(exe, args, workingDir, hideWindow); err != nil {
		slogWin().Debug("RunAsDesktopUser failed", "exe", exe, "err", err)
		return false
	}
	return true
}
