//go:build windows

package winutil

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"TcNo-Acc-Switcher/internal/crashlog"
)

// Start launches exe with args. Uses PowerShell Start-Process -Verb RunAs when opts.Admin.
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
	cmd := exec.Command(exe, args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: opts.HideWindow}
	if err := cmd.Start(); err != nil {
		slogWin().Warn("start failed", "exe", exe, "err", err)
		return WrapIfElevationRequired(err)
	}
	slogWin().Debug("start launched", "exe", exe, "pid", cmd.Process.Pid)
	go func() {
		defer crashlog.Capture()
		_ = cmd.Wait()
	}()
	return nil
}

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

// startAsDesktopUser avoids inheriting admin when the switcher is elevated.
// Prefer CreateProcessWithTokenW (shell user token); fall back to cmd /c start if that fails.
func startAsDesktopUser(exe string, args []string, opts StartOpts) error {
	wd := strings.TrimSpace(opts.WorkingDir)
	if tryRunAsDesktopUser(exe, args, wd, opts.HideWindow) {
		return nil
	}
	slogWin().Debug("falling back to cmd start", "exe", exe)
	cmdline := append([]string{"/c", "start", "", exe}, args...)
	cmd := exec.Command("cmd.exe", cmdline...)
	if wd != "" {
		cmd.Dir = wd
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: opts.HideWindow}
	return WrapIfElevationRequired(cmd.Start())
}
