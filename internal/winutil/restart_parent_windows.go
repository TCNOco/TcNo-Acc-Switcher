//go:build windows

package winutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const restartParentPrefix = "--tcno-restart-parent="

func restartArgs(args []string, parentPID int) []string {
	return append([]string{restartParentPrefix + strconv.Itoa(parentPID)}, args...)
}

// WaitForRestartParent consumes the internal restart argument and waits for the
// old process to exit, releasing both its app and Wails single-instance locks.
// Call before CLI parsing and before initializing any shared application state.
func WaitForRestartParent(args []string) ([]string, error) {
	if len(args) == 0 || !strings.HasPrefix(args[0], restartParentPrefix) {
		return args, nil
	}
	pid, err := strconv.ParseUint(strings.TrimPrefix(args[0], restartParentPrefix), 10, 32)
	if err != nil || pid == 0 || pid == uint64(os.Getpid()) {
		return nil, fmt.Errorf("invalid restart parent: %q", args[0])
	}
	if err := waitForRestartProcess(uint32(pid), 30*time.Second); err != nil {
		return nil, err
	}
	return args[1:], nil
}

func waitForRestartProcess(pid uint32, timeout time.Duration) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err == windows.ERROR_INVALID_PARAMETER {
		// The parent exited before this process reached main.
		return nil
	}
	if err != nil {
		return fmt.Errorf("open restart parent %d: %w", pid, err)
	}
	defer windows.CloseHandle(h)
	state, err := windows.WaitForSingleObject(h, uint32(timeout/time.Millisecond))
	if err != nil {
		return fmt.Errorf("wait for restart parent %d: %w", pid, err)
	}
	if state != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("restart parent %d did not exit within %s (wait=%d)", pid, timeout, state)
	}
	return nil
}
