//go:build windows

package winutil

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var modadvapi32Admin = windows.NewLazySystemDLL("advapi32.dll")
var procCheckTokenMembership = modadvapi32Admin.NewProc("CheckTokenMembership")

func checkTokenMembershipWin(token windows.Token, sid *windows.SID, isMember *int32) error {
	r0, _, err := procCheckTokenMembership.Call(uintptr(token), uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(isMember)))
	if r0 == 0 {
		if err != nil {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}

// CanKillProcesses returns whether the current process can perform KillByName on every entry,
// mirroring C# Globals.CanKillProcess + GeneralFuncs rules for SERVICE: vs TaskKill.
// When ok is false, blocker is the first process/service name that requires elevation.
func CanKillProcesses(names []string, method ClosingMethod) (blocker string, ok bool) {
	if len(names) == 0 {
		return "", true
	}
	if IsProcessElevated() {
		return "", true
	}
	m := method
	if m == "" {
		m = ClosingCombined
	}
	for _, raw := range names {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(raw), strings.ToUpper(servicePrefix)) {
			if m == ClosingTaskKill {
				continue
			}
			// Steam Client Service: stopping via SCM often isn't required to switch Steam accounts;
			// legacy C# skipped admin for this when using TaskKill semantics. Do not force elevation
			// for the proactive check or swap preflight (KillByName still tries SCM then falls back).
			svcTail := strings.TrimSpace(raw[len(servicePrefix):])
			if strings.EqualFold(svcTail, "Steam Client Service") {
				continue
			}
			return raw, false
		}
		base := filepath.Base(raw)
		image := base
		if !strings.HasSuffix(strings.ToLower(base), ".exe") {
			image = strings.TrimSpace(raw) + ".exe"
		}
		if blocked, n := processImageRequiresElevationToKill(image); blocked {
			return n, false
		}
	}
	return "", true
}

func processImageRequiresElevationToKill(image string) (blocked bool, name string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return false, ""
	}
	pid, found, err := firstPIDForImageName(image)
	if err != nil || !found {
		return false, ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return true, image
	}
	defer windows.CloseHandle(h)

	var tok windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok); err != nil {
		return true, image
	}
	defer tok.Close()

	if tok.IsElevated() {
		return true, image
	}
	adminSid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err == nil {
		var isMember int32
		if err := checkTokenMembershipWin(tok, adminSid, &isMember); err == nil && isMember != 0 {
			return true, image
		}
	}
	return false, ""
}

func firstPIDForImageName(want string) (pid uint32, found bool, err error) {
	want = strings.TrimSpace(want)
	if want == "" {
		return 0, false, nil
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, false, err
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		if err == windows.ERROR_NO_MORE_FILES {
			return 0, false, nil
		}
		return 0, false, err
	}
	for {
		exe := utf16FixedToString(pe.ExeFile[:])
		if strings.EqualFold(exe, want) {
			return pe.ProcessID, true, nil
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return 0, false, nil
			}
			return 0, false, err
		}
	}
}
