//go:build windows

package winutil

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExtractExeIcon writes a PNG of the associated icon for exePath to outPNG.
// Uses .NET System.Drawing via PowerShell (reliable for PE/ICO extraction on Windows).
func ExtractExeIcon(exePath, outPNG string) error {
	exePath = filepath.Clean(exePath)
	if st, err := os.Stat(exePath); err != nil || st.IsDir() {
		return fmt.Errorf("invalid exe: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}
	b64Exe := base64.StdEncoding.EncodeToString([]byte(exePath))
	b64Out := base64.StdEncoding.EncodeToString([]byte(outPNG))
	ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing
$p = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$o = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$ico = [System.Drawing.Icon]::ExtractAssociatedIcon($p)
if ($null -eq $ico) { exit 2 }
$bmp = $ico.ToBitmap()
$bmp.Save($o, [System.Drawing.Imaging.ImageFormat]::Png)
`, b64Exe, b64Out)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", ps)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract icon: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
