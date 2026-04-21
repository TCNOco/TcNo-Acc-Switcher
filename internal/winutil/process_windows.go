//go:build windows

package winutil

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const servicePrefix = "SERVICE:"

// KillByName terminates processes by image name (e.g. "steam.exe") or stops Windows services
// when the name is prefixed with SERVICE:.
func KillByName(names []string, method ClosingMethod) error {
	if len(names) == 0 {
		return nil
	}
	m := method
	if m == "" {
		m = ClosingCombined
	}
	for _, raw := range names {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(raw), strings.ToUpper(servicePrefix)) {
			svcName := strings.TrimSpace(raw[len(servicePrefix):])
			if err := stopWindowsService(svcName); err != nil {
				_ = taskKillIM(svcName+".exe", true)
			}
			continue
		}
		base := filepath.Base(raw)
		if !strings.HasSuffix(strings.ToLower(base), ".exe") {
			base = raw + ".exe"
		}
		switch m {
		case ClosingTaskKill:
			_ = taskKillIM(base, true)
		case ClosingClose:
			_ = taskKillIM(base, false)
			time.Sleep(2 * time.Second)
			_ = taskKillIM(base, true)
		default: // Combined
			_ = taskKillIM(base, false)
			time.Sleep(1500 * time.Millisecond)
			_ = taskKillIM(base, true)
		}
	}
	return nil
}

func stopWindowsService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()
	_, err = s.Control(svc.Stop)
	return err
}

func taskKillIM(name string, force bool) error {
	args := []string{"/C", "taskkill"}
	if force {
		args = append(args, "/F")
	}
	args = append(args, "/T", "/IM", name)
	cmd := exec.Command("cmd.exe", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if strings.Contains(s, "not running") || strings.Contains(s, "could not find") || strings.Contains(s, "not found") {
			return nil
		}
		return fmt.Errorf("taskkill: %w: %s", err, s)
	}
	return nil
}

// IsProcessElevated returns true if the current process is running elevated (admin).
func IsProcessElevated() bool {
	var tok windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok)
	if err != nil {
		return false
	}
	defer tok.Close()
	return tok.IsElevated()
}

// Start launches exe with args. Uses PowerShell Start-Process -Verb RunAs when opts.Admin.
func Start(exe string, args []string, opts StartOpts) error {
	if opts.AsDesktopUser && IsProcessElevated() {
		return startAsDesktopUser(exe, args, opts)
	}
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return fmt.Errorf("empty executable")
	}
	if opts.Admin {
		return startElevated(exe, args, opts)
	}
	cmd := exec.Command(exe, args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: opts.HideWindow}
	return cmd.Start()
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
// Uses cmd /c start "" exe args — may not cover all cases; full WTS token path is a future improvement.
func startAsDesktopUser(exe string, args []string, opts StartOpts) error {
	cmdline := append([]string{"/c", "start", "", exe}, args...)
	cmd := exec.Command("cmd.exe", cmdline...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: opts.HideWindow}
	return cmd.Start()
}

// StartAsDesktopUser is exported for callers that always want non-inherited launch.
func StartAsDesktopUser(exe string, args []string, opts StartOpts) error {
	opts.AsDesktopUser = true
	return Start(exe, args, opts)
}
