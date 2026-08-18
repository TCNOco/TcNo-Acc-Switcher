//go:build windows

package winutil

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func normalizeExeBase(s string) string {
	s = strings.TrimSpace(filepath.Base(s))
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(s), ".exe") {
		s += ".exe"
	}
	return s
}

func snapshotProcesses() ([]procLite, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		if err == windows.ERROR_NO_MORE_FILES {
			return nil, nil
		}
		return nil, err
	}
	var out []procLite
	for {
		exe := utf16FixedToString(pe.ExeFile[:])
		out = append(out, procLite{
			PID:       pe.ProcessID,
			ParentPID: pe.ParentProcessID,
			ExeBase:   exe,
		})
		if err := windows.Process32Next(snap, &pe); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return out, nil
			}
			return out, err
		}
	}
}

func chromiumBrowserRootPIDs(wantExe string, all []procLite) []uint32 {
	want := normalizeExeBase(wantExe)
	if want == "" || len(all) == 0 {
		return nil
	}
	sameImage := make(map[uint32]bool)
	for _, p := range all {
		if strings.EqualFold(p.ExeBase, want) {
			sameImage[p.PID] = true
		}
	}
	var roots []uint32
	for _, p := range all {
		if !strings.EqualFold(p.ExeBase, want) {
			continue
		}
		if !sameImage[p.ParentPID] {
			roots = append(roots, p.PID)
		}
	}
	if len(roots) == 0 {
		for _, p := range all {
			if strings.EqualFold(p.ExeBase, want) {
				roots = append(roots, p.PID)
			}
		}
	}
	return roots
}

func exeBaseForPIDInSnapshot(all []procLite, pid uint32) string {
	for _, p := range all {
		if p.PID == pid {
			return p.ExeBase
		}
	}
	return ""
}

func allPIDsForImageName(want string) ([]uint32, error) {
	want = normalizeExeBase(want)
	if want == "" {
		return nil, nil
	}
	all, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	var out []uint32
	for _, p := range all {
		if strings.EqualFold(p.ExeBase, want) {
			out = append(out, p.PID)
		}
	}
	return out, nil
}

func processExistsByImageName(want string) (bool, error) {
	pids, err := allPIDsForImageName(want)
	if err != nil {
		return false, err
	}
	return len(pids) > 0, nil
}

// SnapshotRunningExeBasenames returns lowercase exe base names for all running processes (e.g. cs2.exe).
func SnapshotRunningExeBasenames() (map[string]struct{}, error) {
	all, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(all))
	for _, p := range all {
		base := normalizeExeBase(p.ExeBase)
		if base == "" {
			continue
		}
		out[strings.ToLower(base)] = struct{}{}
	}
	return out, nil
}

// IsExeRunning reports whether any process with the given image base name is running.
func IsExeRunning(exeName string) bool {
	ok, err := processExistsByImageName(exeName)
	return err == nil && ok
}

// SnapshotMatchingPIDs maps the PID of every running process whose lowercase image
// base name is in want to that name. One pass over the process table, for callers
// that need a starting point before switching to event-driven tracking.
func SnapshotMatchingPIDs(want map[string]struct{}) (map[uint32]string, error) {
	if len(want) == 0 {
		return nil, nil
	}
	all, err := snapshotProcesses()
	if err != nil {
		return nil, err
	}
	out := make(map[uint32]string)
	for _, p := range all {
		base := strings.ToLower(normalizeExeBase(p.ExeBase))
		if base == "" {
			continue
		}
		if _, ok := want[base]; ok {
			out[p.PID] = base
		}
	}
	return out, nil
}

func utf16FixedToString(b []uint16) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return windows.UTF16ToString(b[:n])
}

// waitForImageExit polls until exeImage is gone or maxWait elapses.
func waitForImageExit(exeImage string, maxWait, poll time.Duration, targetCount int) {
	deadline := time.Now().Add(maxWait)
	start := time.Now()
	lastReported := -1
	reportWaitStatus := func() {
		elapsed := int(time.Since(start).Seconds())
		if elapsed == lastReported {
			return
		}
		lastReported = elapsed
		key := "Status_WaitingForClose"
		vars := map[string]string{
			"processName": exeImage,
			"timeout":     fmt.Sprint(elapsed),
			"timeLimit":   fmt.Sprint(int(maxWait.Seconds())),
		}
		if targetCount > 1 {
			key = "Status_WaitingForMultipleClose"
			vars["count"] = fmt.Sprint(targetCount - 1)
		}
		emitStatus(key, vars)
	}
	pids, err := allPIDsForImageName(exeImage)
	if err != nil || len(pids) == 0 {
		return
	}

	// Wait on the real process handles instead of re-reading the process table. The wait
	// returns the instant the last process dies, so poll no longer bounds how fast we notice
	// an exit - it only bounds how often the "waiting for X" status line is refreshed.
	var handles []windows.Handle
	var unwaitable []uint32
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err != nil {
			// ERROR_ACCESS_DENIED means it is alive but we cannot wait on it; anything
			// else means it is already gone.
			if err == windows.ERROR_ACCESS_DENIED {
				unwaitable = append(unwaitable, pid)
			}
			continue
		}
		handles = append(handles, h)
	}
	defer func() {
		for _, h := range handles {
			windows.CloseHandle(h)
		}
	}()

	if poll <= 0 {
		poll = 100 * time.Millisecond
	}
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		chunk := poll
		if chunk > remaining {
			chunk = remaining
		}
		if len(handles) > 0 {
			// WaitForMultipleObjects caps at 64 handles; wait them down in batches. With
			// waitAll set, a batch returns as soon as every handle in it is signalled.
			batch := handles
			if len(batch) > maxWaitObjects {
				batch = batch[:maxWaitObjects]
			}
			r, err := windows.WaitForMultipleObjects(batch, true, uint32(chunk/time.Millisecond))
			if err == nil && r != uint32(windows.WAIT_TIMEOUT) {
				handles = handles[len(batch):]
				continue
			}
		} else if len(unwaitable) > 0 {
			time.Sleep(chunk)
		}
		if len(handles) == 0 {
			if len(unwaitable) == 0 {
				return
			}
			if live, err := allPIDsForImageName(exeImage); err == nil && len(live) == 0 {
				return
			}
		}
		reportWaitStatus()
	}
}

// waitForElectronImageExit polls often so we return immediately once the image is gone; we do not
// send extra taskkill nudges mid-wait (those can interrupt a partially finished graceful shutdown).
func waitForElectronImageExit(exeImage string, maxWait time.Duration, targetCount int) {
	waitForImageExit(exeImage, maxWait, electronPollInterval, targetCount)
}
