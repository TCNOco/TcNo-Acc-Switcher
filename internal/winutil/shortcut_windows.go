//go:build windows

package winutil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type lnkShellJSON struct {
	TargetPath   string `json:"TargetPath"`
	Arguments    string `json:"Arguments"`
	IconLocation string `json:"IconLocation"`
}

// ReadLnkShortcut returns target exe, arguments, and icon location (e.g. "path,0") from a .lnk file.
func ReadLnkShortcut(lnkPath string) (target string, arguments string, iconLocation string, err error) {
	if lnkPath == "" {
		return "", "", "", fmt.Errorf("empty shortcut path")
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(lnkPath))
	ps := fmt.Sprintf(
		"$p=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'));"+
			" $s=(New-Object -ComObject WScript.Shell).CreateShortcut($p);"+
			" [pscustomobject]@{ TargetPath=$s.TargetPath; Arguments=$s.Arguments; IconLocation=$s.IconLocation } | ConvertTo-Json -Compress",
		b64,
	)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", ps).CombinedOutput()
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	raw := strings.TrimSpace(string(out))
	var j lnkShellJSON
	if uerr := json.Unmarshal([]byte(raw), &j); uerr == nil {
		return j.TargetPath, j.Arguments, j.IconLocation, nil
	}
	// Fallback for unexpected PS output (warnings prefixed, etc.)
	lines := strings.Split(raw, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		if uerr := json.Unmarshal([]byte(line), &j); uerr == nil {
			return j.TargetPath, j.Arguments, j.IconLocation, nil
		}
	}
	return "", "", "", fmt.Errorf("shortcut json: parse failed (output=%q)", raw)
}
