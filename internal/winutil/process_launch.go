//go:build windows

package winutil

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// Start launches exe with args, fully decoupled from this process (see [spawnDetached]).
// Uses PowerShell Start-Process -Verb RunAs when opts.Admin.
func Start(exe string, args []string, opts StartOpts) error {
	if opts.AsDesktopUser && IsProcessElevated() {
		slogWin().Debug("start request", "exe", exe, "mode", "desktop-user", "args", len(args), "admin", opts.Admin, "method", opts.Method)
		return startAsDesktopUser(exe, args, opts)
	}
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
	slogWin().Debug("start request", "exe", exe, "args", len(args), "admin", opts.Admin, "method", opts.Method, "workingDir", strings.TrimSpace(opts.WorkingDir))
	if opts.Admin {
		err := startElevated(exe, args, opts)
		if err != nil {
			slogWin().Warn("start failed", "exe", exe, "err", err)
			return err
		}
		slogWin().Debug("start launched", "exe", exe, "mode", "elevated")
		return nil
	}
	pid, err := spawnDetached(exe, args, opts.WorkingDir, opts.HideWindow)
	if err != nil {
		slogWin().Warn("start failed", "exe", exe, "err", err)
		return WrapIfElevationRequired(err)
	}
	slogWin().Debug("start launched", "exe", exe, "pid", pid)
	return nil
}

// startElevated launches through the UAC broker: AppInfo creates the target, so the elevated
// program is never our child and never in our job. Only the PowerShell shim is, and that gets
// its own console and job breakaway so a crash here cannot abort a launch in progress.
func startElevated(exe string, args []string, opts StartOpts) error {
	var b strings.Builder
	b.WriteString(`Start-Process -FilePath `)
	b.WriteString(fmt.Sprintf("%q", exe))
	if len(args) > 0 {
		b.WriteString(` -ArgumentList `)
		b.WriteString(psArgList(args))
	}
	if wd := strings.TrimSpace(opts.WorkingDir); wd != "" {
		b.WriteString(` -WorkingDirectory `)
		b.WriteString(fmt.Sprintf("%q", wd))
	}
	b.WriteString(` -Verb RunAs`)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", b.String())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: helperCreationFlags()}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start elevated: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func psArgList(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("@(")
	for i, a := range args {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("'")
		b.WriteString(strings.ReplaceAll(a, "'", "''"))
		b.WriteString("'")
	}
	b.WriteString(")")
	return b.String()
}

// startAsDesktopUser avoids inheriting admin when the switcher is elevated. Reparenting to the
// shell drops elevation and leaves our process tree in one step, so it is tried first;
// CreateProcessWithTokenW (shell user token) and cmd /c start remain as fallbacks. Every step
// here runs as the desktop user - none may quietly degrade into an elevated launch.
func startAsDesktopUser(exe string, args []string, opts StartOpts) error {
	wd := strings.TrimSpace(opts.WorkingDir)
	if pid, err := spawnReparented(exe, args, wd, opts.HideWindow); err == nil {
		slogWin().Debug("start launched", "exe", exe, "mode", "shell-reparented", "pid", pid)
		return nil
	} else {
		slogWin().Debug("shell reparent unavailable", "exe", exe, "err", err)
	}
	if tryRunAsDesktopUser(exe, args, wd, opts.HideWindow) {
		return nil
	}
	slogWin().Debug("falling back to cmd start", "exe", exe)
	cmdline := append([]string{"/c", "start", "", exe}, args...)
	// The shim is always hidden regardless of opts.HideWindow: `start` (no /B) builds its own
	// STARTUPINFO for the target, so cmd's console is a separate window that only ever shows up
	// as a stray popup - worse under Windows Terminal, which keeps it around.
	if _, err := spawnDetached("cmd.exe", cmdline, wd, true); err != nil {
		return WrapIfElevationRequired(err)
	}
	return nil
}
