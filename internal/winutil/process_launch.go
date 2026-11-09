//go:build windows

package winutil

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"TcNo-Acc-Switcher/internal/actionlog"
)

// Start launches exe with args, fully decoupled from this process (see [spawnDetached]).
// Uses PowerShell Start-Process -Verb RunAs when opts.Admin.
func Start(exe string, args []string, opts StartOpts) (err error) {
	appElevated := IsProcessElevated()
	mode := "detached"
	if opts.AsDesktopUser && appElevated {
		mode = "desktop_user"
	} else if opts.Admin {
		mode = "elevated"
	}
	// Launch arguments can contain credentials. Only record the mode and count.
	defer func() {
		recordErr := err
		if opts.Admin && err != nil {
			// PowerShell's error output may echo the full command, including arguments.
			recordErr = fmt.Errorf("elevated launch failed")
		}
		actionlog.Record("process:start", exe, fmt.Sprintf("app_elevated=%t run_as_admin=%t as_desktop_user=%t launch_mode=%s argument_count=%d", appElevated, opts.Admin, opts.AsDesktopUser, mode, len(args)), recordErr)
	}()
	slogWin().Debug("start request", "exe", exe, "launch_mode", mode, "app_elevated", appElevated,
		"run_as_admin", opts.Admin, "as_desktop_user", opts.AsDesktopUser, "args", len(args), "workingDir", strings.TrimSpace(opts.WorkingDir))
	if opts.AsDesktopUser && appElevated {
		return startAsDesktopUser(exe, args, opts)
	}
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
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

// Ask the running desktop shell to create the process with its own token and environment.
// Merely assigning Explorer as the parent still creates the target from our elevated
// context. Keep the explicit desktop-token launch as a fallback, never an elevated cmd shim.
func startAsDesktopUser(exe string, args []string, opts StartOpts) error {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
	wd := strings.TrimSpace(opts.WorkingDir)
	return launchAsDesktopUser(func() error {
		return shellExecuteAsDesktopUser(exe, args, wd, opts.HideWindow)
	}, func() error {
		return runAsDesktopUser(exe, args, wd, opts.HideWindow)
	}, func(mode string, err error) {
		actionlog.Record("process:desktop_launch", exe, mode, err)
	})
}

func launchAsDesktopUser(shell, token func() error, record func(string, error)) error {
	shellErr := shell()
	record("explorer", shellErr)
	if shellErr == nil {
		return nil
	}
	tokenErr := token()
	record("desktop_token", tokenErr)
	if tokenErr == nil {
		return nil
	}
	return fmt.Errorf("launch as desktop user: %w", errors.Join(
		fmt.Errorf("Explorer: %w", shellErr), fmt.Errorf("desktop token: %w", tokenErr)))
}
