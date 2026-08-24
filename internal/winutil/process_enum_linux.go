//go:build linux

package winutil

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// procNameMaxLen is TASK_COMM_LEN-1: /proc/<pid>/comm holds 15 characters.
const procNameMaxLen = 15

func forEachProcess(fn func(pid int, names []string)) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		slogWin().Warn("read /proc", "err", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		var names []string
		// A process that exits mid-walk simply has no names to match.
		if comm, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm")); err == nil {
			if c := strings.TrimSpace(string(comm)); c != "" {
				names = append(names, c)
			}
		}
		if exe, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe")); err == nil {
			if base := filepath.Base(strings.TrimSuffix(exe, " (deleted)")); base != "" && base != "." {
				names = append(names, base)
			}
		}
		if len(names) == 0 {
			continue
		}
		fn(pid, names)
	}
}
