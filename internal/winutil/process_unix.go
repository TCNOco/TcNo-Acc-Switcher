//go:build unix

package winutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// KillByNameWithOpts ends a platform with native quit, then SIGTERM, then SIGKILL.
//
// Close and Electron name Windows mechanics - a WM_CLOSE broadcast, synthesised
// Alt+F4 - with no counterpart here, so every method except TaskKill collapses
// to that one escalation.
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

// unixKillTargets reshapes the catalog's Windows process names for a Unix
// process table: "steam.exe" is "steam" here, and SERVICE: entries name Windows
// services that have no equivalent.
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

// unixMatchingPIDs maps live PIDs to the target they matched. Our own process is
// never a candidate: a platform image sharing our name would have us signal
// ourselves.
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

// unixNameMatches compares a process's names against one target.
//
// A kernel-reported process name is truncated - to 15 characters on Linux, 16 on
// macOS - so a longer target is also compared against its own truncation.
// Otherwise a process whose full path is unreadable never matches.
func unixNameMatches(names []string, target string) bool {
	truncated := target
	if len(truncated) > procNameMaxLen {
		truncated = truncated[:procNameMaxLen]
	}
	for _, name := range names {
		if strings.EqualFold(name, target) || strings.EqualFold(name, truncated) {
			return true
		}
	}
	return false
}

// WaitForegroundForExe has no Unix counterpart.
func WaitForegroundForExe(_ string, _ time.Duration) bool {
	return false
}

// Start launches exe in its own session and releases it, so the platform
// outlives the switcher and a switcher crash cannot take it down.
//
// opts.Admin and opts.AsDesktopUser are Windows elevation concepts and are
// ignored: there is no UAC to satisfy, and launching a game platform through
// sudo would leave root-owned files in the user's home.
func Start(exe string, args []string, opts StartOpts) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
	if opts.Admin {
		slogWin().Debug("ignoring admin launch request", "exe", exe)
	}
	slogWin().Debug("start request", "exe", exe, "args", len(args), "workingDir", strings.TrimSpace(opts.WorkingDir))

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

func IsProcessElevated() bool {
	return os.Geteuid() == 0
}

// StartAsDesktopUser has no Unix counterpart: Start never elevates, so there is
// no elevation to drop.
func StartAsDesktopUser(exe string, args []string, opts StartOpts) error {
	return ErrUnsupported
}

// SnapshotRunningExeBasenames returns running executable names, lowercased.
func SnapshotRunningExeBasenames() (map[string]struct{}, error) {
	out := map[string]struct{}{}
	forEachProcess(func(_ int, names []string) {
		for _, name := range names {
			out[strings.ToLower(name)] = struct{}{}
		}
	})
	return out, nil
}

func IsExeRunning(name string) bool {
	targets := unixKillTargets([]string{name})
	if len(targets) == 0 {
		return false
	}
	return len(unixMatchingPIDs(targets)) > 0
}

// SnapshotMatchingPIDs maps live PIDs to the lowercased name they matched.
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
