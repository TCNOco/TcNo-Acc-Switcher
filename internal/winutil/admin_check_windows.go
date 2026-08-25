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
	var pids map[string]uint32
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
		// One walk of the process table answers for every name. Taken lazily, so
		// a list of nothing but SERVICE: entries never pays for it.
		if pids == nil {
			p, err := snapshotFirstPIDs()
			if err != nil {
				// Same outcome as before, when each name swallowed this error
				// separately: nothing can be shown to need elevation.
				return "", true
			}
			pids = p
		}
		if blocked, n := processImageRequiresElevationToKill(image, pids); blocked {
			return n, false
		}
	}
	return "", true
}

func processImageRequiresElevationToKill(image string, pids map[string]uint32) (blocked bool, name string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return false, ""
	}
	pid, found := pids[strings.ToLower(image)]
	if !found {
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

// snapshotFirstPIDs walks the process table once and records the first PID seen
// for each image name, lowercased.
//
// This used to be one snapshot per name looked up, and CanKillProcesses asks
// about every executable a platform lists - so a platform naming six of them
// walked the same process table six times.
func snapshotFirstPIDs() (map[string]uint32, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	out := map[string]uint32{}
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		if err == windows.ERROR_NO_MORE_FILES {
			return out, nil
		}
		return nil, err
	}
	for {
		// First wins, matching the enumeration order the per-name scan returned.
		if exe := strings.ToLower(utf16FixedToString(pe.ExeFile[:])); exe != "" {
			if _, seen := out[exe]; !seen {
				out[exe] = pe.ProcessID
			}
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return out, nil
			}
			return nil, err
		}
	}
}
