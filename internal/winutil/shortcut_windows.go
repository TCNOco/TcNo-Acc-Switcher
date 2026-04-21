//go:build windows

package winutil

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

// ReadLnkShortcut returns target exe, arguments, and icon location (e.g. "path,0") from a .lnk file.
func ReadLnkShortcut(lnkPath string) (target string, arguments string, iconLocation string, err error) {
	if lnkPath == "" {
		return "", "", "", fmt.Errorf("empty shortcut path")
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(lnkPath))
	ps := fmt.Sprintf(
		"$p=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s')); $s=(New-Object -ComObject WScript.Shell).CreateShortcut($p); $s.TargetPath; $s.Arguments; $s.IconLocation",
		b64,
	)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", ps).CombinedOutput()
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	switch len(lines) {
	case 0:
		return "", "", "", fmt.Errorf("empty shortcut output")
	case 1:
		return lines[0], "", "", nil
	case 2:
		return lines[0], lines[1], "", nil
	default:
		return lines[0], lines[1], lines[2], nil
	}
}
