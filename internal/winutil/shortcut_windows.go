//go:build windows

package winutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WriteShortcutLnk creates a Windows shell shortcut via PowerShell COM (best-effort).
func WriteShortcutLnk(shortcutPath, targetExe, arguments, iconLocation string) error {
	shortcutPath = filepath.Clean(shortcutPath)
	targetExe = strings.TrimSpace(targetExe)
	if shortcutPath == "" || targetExe == "" {
		return fmt.Errorf("shortcut path or target empty")
	}
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0o755); err != nil {
		return err
	}
	ps := fmt.Sprintf("$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%s'); $s.TargetPath='%s'; $s.Arguments='%s'",
		psEscape(shortcutPath), psEscape(targetExe), psEscape(arguments))
	if strings.TrimSpace(iconLocation) != "" {
		ps += fmt.Sprintf("; $s.IconLocation='%s'", psEscape(iconLocation))
	}
	ps += "; $s.Save()"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", ps)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func psEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// ReadLnkShortcut returns target exe, arguments, and icon location (e.g. "path,0") from a .lnk file.
func ReadLnkShortcut(lnkPath string) (target string, arguments string, iconLocation string, err error) {
	if lnkPath == "" {
		return "", "", "", fmt.Errorf("empty shortcut path")
	}
	b, err := os.ReadFile(lnkPath)
	if err != nil {
		return "", "", "", err
	}
	return parseLnk(b)
}
