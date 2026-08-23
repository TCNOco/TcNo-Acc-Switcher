//go:build linux

package winutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const servicePrefix = "SERVICE:"

const (
	unixNativeQuitWait = 12 * time.Second
	unixTermWait       = 5 * time.Second
	unixKillWait       = 2 * time.Second
	unixPollInterval   = 100 * time.Millisecond
)

// KillByName terminates every process whose executable matches one of names.
func KillByName(names []string, method ClosingMethod, beforeElectronSynth func() error) error {
	return KillByNameWithOpts(names, method, KillOpts{BeforeElectronSynth: beforeElectronSynth})
}

// KillByNameWithOpts ends a platform the way KillByNameWithOpts does on Windows, but with the
// signals a Unix process table understands.
//
// Every ClosingMethod except TaskKill collapses to the same escalation here - native quit, then
// SIGTERM, then SIGKILL. Close and Electron name Windows-only mechanics (a WM_CLOSE broadcast
// and synthesised Alt+F4) that have no counterpart on Linux, and pretending otherwise would only
// mean burning their deadlines before arriving at the signal that was always going to do it.
func KillByNameWithOpts(names []string, method ClosingMethod, opts KillOpts) error {
	targets := unixKillTargets(names)
	if len(targets) == 0 {
		return nil
	}
	m := method
	if m == "" {
		m = ClosingCombined
	}
	slogWin().Debug("kill begin", "method", m, "targets", len(targets), "native", opts.NativeQuit != nil)

	if m != ClosingTaskKill {
		if opts.NativeQuit != nil {
			if err := opts.NativeQuit(); err != nil {
				slogWin().Warn("native quit failed; falling back to signals", "err", err)
			} else if waitForUnixExit(targets, unixNativeQuitWait) {
				slogWin().Debug("kill completed", "method", m, "via", "native-quit")
				return nil
			}
		}
		signalUnixTargets(targets, syscall.SIGTERM)
		if waitForUnixExit(targets, unixTermWait) {
			slogWin().Debug("kill completed", "method", m, "via", "sigterm")
			return nil
		}
	}

	signalUnixTargets(targets, syscall.SIGKILL)
	waitForUnixExit(targets, unixKillWait)
	slogWin().Debug("kill completed", "method", m, "via", "sigkill")
	return nil
}

// unixKillTargets reshapes the catalog's Windows process names for a Unix process table.
// Platforms.json names images as Windows sees them ("steam.exe"); the same program is "steam"
// here. SERVICE: entries name Windows services and are dropped - the Linux Steam has no
// equivalent of the Steam Client Service.
func unixKillTargets(names []string) []string {
	var out []string
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(name), servicePrefix) {
			slogWin().Debug("skipping Windows service target", "name", name)
			continue
		}
		name = filepath.Base(name)
		if len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".exe") {
			name = name[:len(name)-4]
		}
		if name == "" || name == "." || name == string(filepath.Separator) {
			continue
		}
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

func signalUnixTargets(targets []string, sig syscall.Signal) {
	for pid, name := range unixMatchingPIDs(targets) {
		if err := syscall.Kill(pid, sig); err != nil && err != syscall.ESRCH {
			slogWin().Warn("signal failed", "process", name, "pid", pid, "signal", sig.String(), "err", err)
			continue
		}
		slogWin().Debug("signalled", "process", name, "pid", pid, "signal", sig.String())
	}
}

func waitForUnixExit(targets []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if len(unixMatchingPIDs(targets)) == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(unixPollInterval)
	}
}

// unixMatchingPIDs maps every live PID whose executable matches a target to the name it matched.
// The switcher's own process is never a candidate: a platform image sharing our name would
// otherwise have us signal ourselves.
func unixMatchingPIDs(targets []string) map[int]string {
	out := map[int]string{}
	if len(targets) == 0 {
		return out
	}
	self := os.Getpid()
	forEachProcess(func(pid int, names []string) {
		if pid == self || pid == 1 {
			return
		}
		for _, target := range targets {
			if unixNameMatches(names, target) {
				out[pid] = target
				return
			}
		}
	})
	return out
}

// forEachProcess walks /proc and hands each live PID the names it can be recognised by:
// its comm value and the basename of its executable.
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

// unixNameMatches compares a process's names against one target. /proc/<pid>/comm is truncated
// to 15 characters, so a longer target has to be compared against its own truncation or a
// process whose executable link is unreadable would never match.
func unixNameMatches(names []string, target string) bool {
	truncated := target
	if len(truncated) > 15 {
		truncated = truncated[:15]
	}
	for _, name := range names {
		if strings.EqualFold(name, target) || strings.EqualFold(name, truncated) {
			return true
		}
	}
	return false
}

// WaitForegroundForExe has no Linux counterpart; the window managers the switcher would have to
// ask are not a single API here.
func WaitForegroundForExe(_ string, _ time.Duration) bool {
	return false
}

// Start launches exe detached from this process: its own session, no inherited standard streams,
// and released so it is never reaped as our child. A platform must outlive the switcher, and a
// switcher crash must not take the platform with it.
//
// opts.Admin and opts.AsDesktopUser are Windows elevation concepts and are ignored. There is no
// UAC to satisfy, and launching a game platform through sudo/pkexec would leave root-owned files
// in the user's home - worse than the setting not applying.
func Start(exe string, args []string, opts StartOpts) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
	if opts.Admin {
		slogWin().Debug("ignoring admin launch request on linux", "exe", exe)
	}
	slogWin().Debug("start request", "exe", exe, "args", len(args), "method", opts.Method, "workingDir", strings.TrimSpace(opts.WorkingDir))

	cmd := exec.Command(exe, args...)
	cmd.Dir = strings.TrimSpace(opts.WorkingDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		slogWin().Warn("start failed", "exe", exe, "err", err)
		return err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		slogWin().Warn("release launched process", "exe", exe, "pid", pid, "err", err)
	}
	slogWin().Debug("start launched", "exe", exe, "pid", pid)
	return nil
}

// IsProcessElevated reports whether the switcher is running as root.
func IsProcessElevated() bool {
	return os.Geteuid() == 0
}

// StartAsDesktopUser has no Linux counterpart: Start never elevates, so there is no elevation to
// drop on the way to the desktop session.
func StartAsDesktopUser(exe string, args []string, opts StartOpts) error {
	return ErrUnsupported
}

// SnapshotRunningExeBasenames returns the set of executable names currently running, lowercased
// so callers can compare against catalog names regardless of case.
func SnapshotRunningExeBasenames() (map[string]struct{}, error) {
	out := map[string]struct{}{}
	forEachProcess(func(_ int, names []string) {
		for _, name := range names {
			out[strings.ToLower(name)] = struct{}{}
		}
	})
	return out, nil
}

// IsExeRunning reports whether any live process matches the given image name.
func IsExeRunning(name string) bool {
	targets := unixKillTargets([]string{name})
	if len(targets) == 0 {
		return false
	}
	return len(unixMatchingPIDs(targets)) > 0
}

// SnapshotMatchingPIDs maps live PIDs to the lowercased name they matched, for the names in want.
func SnapshotMatchingPIDs(want map[string]struct{}) (map[uint32]string, error) {
	if len(want) == 0 {
		return nil, nil
	}
	targets := make([]string, 0, len(want))
	for name := range want {
		targets = append(targets, name)
	}
	targets = unixKillTargets(targets)

	out := map[uint32]string{}
	for pid, name := range unixMatchingPIDs(targets) {
		if pid < 0 {
			continue
		}
		out[uint32(pid)] = strings.ToLower(name)
	}
	return out, nil
}
