//go:build windows

package winutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RunValueNameStartupTray is the HKCU Run value used for "start with Windows" (-tray).
const RunValueNameStartupTray = "TcNoAccSwitcher"

const runKeyStartupTray = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run:` + RunValueNameStartupTray

// RunCommandTrayArgs is appended after the quoted executable for startup-tray mode.
const RunCommandTrayArgs = "-tray"

// RunAtStartupTrayCommand returns the full Run registry string: "path\to\exe" -tray
func RunAtStartupTrayCommand(exePath string) string {
	exePath = filepath.Clean(strings.TrimSpace(exePath))
	if exePath == "" {
		return ""
	}
	return fmt.Sprintf(`"%s" `+RunCommandTrayArgs, exePath)
}

// SetRunAtStartupTray registers or removes the current-user Run entry for tray startup.
func SetRunAtStartupTray(exePath string, enabled bool) error {
	if !enabled {
		return RegistryWrite(runKeyStartupTray, "")
	}
	cmd := RunAtStartupTrayCommand(exePath)
	if cmd == "" {
		return fmt.Errorf("empty executable path")
	}
	return RegistryWrite(runKeyStartupTray, cmd)
}

// SyncRunAtStartupTray ensures the Run entry matches want (idempotent).
func SyncRunAtStartupTray(exePath string, want bool) error {
	return SetRunAtStartupTray(exePath, want)
}
