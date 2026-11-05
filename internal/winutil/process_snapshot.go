//go:build windows

package winutil

import (
	"errors"
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

func ProcessImageRunning(names []string) (bool, error) {
	if len(names) == 0 {
		return false, nil
	}
	running, err := SnapshotRunningExeBasenames()
	if err != nil {
		return false, err
	}
	for _, name := range names {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(name)), servicePrefix) {
			continue
		}
		if _, ok := running[strings.ToLower(normalizeExeBase(name))]; ok {
			return true, nil
		}
	}
	return false, nil
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

var errProcessExitTimeout = errors.New("timed out waiting for process exit")

func waitForImageExit(exeImage string, maxWait, poll time.Duration, targetCount int) error {
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
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}
	for {
		pids, err := allPIDsForImageName(exeImage)
		if err != nil {
			return fmt.Errorf("inspect process %s while waiting for exit: %w", exeImage, err)
		}
		if len(pids) == 0 {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("%w: %s", errProcessExitTimeout, exeImage)
		}
		if err := waitForProcessHandles(pids, deadline, poll, reportWaitStatus); err != nil {
			return fmt.Errorf("wait for process %s: %w", exeImage, err)
		}
	}
}

func waitForProcessHandles(pids []uint32, deadline time.Time, poll time.Duration, report func()) error {
	var handles []windows.Handle
	unwaitable := false
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err != nil {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				unwaitable = true
				continue
			}
			if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
				continue
			}
			for _, opened := range handles {
				windows.CloseHandle(opened)
			}
			return fmt.Errorf("open process %d for exit notification: %w", pid, err)
		}
		handles = append(handles, h)
	}
	opened := handles
	defer func() {
		for _, h := range opened {
			windows.CloseHandle(h)
		}
	}()
	for len(handles) > 0 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errProcessExitTimeout
		}
		chunk := poll
		if chunk > remaining {
			chunk = remaining
		}
		batch := handles
		if len(batch) > maxWaitObjects {
			batch = batch[:maxWaitObjects]
		}
		r, err := windows.WaitForMultipleObjects(batch, true, uint32(chunk/time.Millisecond))
		if err != nil {
			return err
		}
		if r == uint32(windows.WAIT_OBJECT_0) {
			handles = handles[len(batch):]
			continue
		}
		if r != uint32(windows.WAIT_TIMEOUT) {
			return fmt.Errorf("unexpected process wait result %d", r)
		}
		report()
	}
	if unwaitable {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errProcessExitTimeout
		}
		if remaining < poll {
			poll = remaining
		}
		time.Sleep(poll)
		report()
	}
	return nil
}

// waitForElectronImageExit polls often so we return immediately once the image is gone; we do not
// send extra taskkill nudges mid-wait (those can interrupt a partially finished graceful shutdown).
func waitForElectronImageExit(exeImage string, maxWait time.Duration, targetCount int) error {
	return waitForImageExit(exeImage, maxWait, electronPollInterval, targetCount)
}
