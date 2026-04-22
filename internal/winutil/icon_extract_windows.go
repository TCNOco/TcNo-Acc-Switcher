//go:build windows

package winutil

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ExtractShortcutIcon writes a PNG for a .lnk or .url shortcut to outPNG.
func ExtractShortcutIcon(shortcutPath, outPNG string) error {
	shortcutPath = filepath.Clean(shortcutPath)
	low := strings.ToLower(shortcutPath)
	switch {
	case strings.HasSuffix(low, ".lnk"):
		return extractLnkIcon(shortcutPath, outPNG)
	case strings.HasSuffix(low, ".url"):
		return extractURLIcon(shortcutPath, outPNG)
	default:
		return ExtractExeIcon(shortcutPath, outPNG)
	}
}

func normWinPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(strings.TrimRight(s, "\x00"))
}

// splitShellIconLocation splits Shell32 IconLocation ("C:\a\b.exe,0" or "C:\a\b.ico").
func splitShellIconLocation(iconLoc string) (path string, index int) {
	iconLoc = normWinPath(iconLoc)
	if iconLoc == "" {
		return "", 0
	}
	idx := strings.LastIndex(iconLoc, ",")
	if idx <= 0 {
		return iconLoc, 0
	}
	suffix := strings.TrimSpace(iconLoc[idx+1:])
	n, err := strconv.Atoi(suffix)
	if err != nil {
		return iconLoc, 0
	}
	return strings.TrimSpace(iconLoc[:idx]), n
}

func extractLnkIcon(lnkPath, outPNG string) error {
	target, _, iconLocRaw, err := ReadLnkShortcut(lnkPath)
	if err != nil {
		return err
	}
	target = normWinPath(target)
	iconPath, iconIdx := splitShellIconLocation(iconLocRaw)

	if iconPath != "" {
		if st, err := os.Stat(iconPath); err == nil && !st.IsDir() {
			ext := strings.ToLower(filepath.Ext(iconPath))
			switch ext {
			case ".exe":
				if iconIdx > 0 {
					if err := extractIconExToPNG(iconPath, iconIdx, outPNG); err == nil {
						return nil
					}
				}
				if err := ExtractExeIcon(iconPath, outPNG); err == nil {
					return nil
				}
				if err := extractIconExToPNG(iconPath, iconIdx, outPNG); err == nil {
					return nil
				}
			case ".ico":
				if err := extractICOToPNG(iconPath, outPNG); err == nil {
					return nil
				}
			case ".dll":
				if err := extractIconExToPNG(iconPath, iconIdx, outPNG); err == nil {
					return nil
				}
			}
		}
	}
	if target != "" {
		if st, err := os.Stat(target); err == nil && !st.IsDir() {
			ext := strings.ToLower(filepath.Ext(target))
			if ext == ".exe" {
				if err := ExtractExeIcon(target, outPNG); err == nil {
					return nil
				}
				if err := extractIconExToPNG(target, 0, outPNG); err == nil {
					return nil
				}
			}
		}
	}
	return writeMinimalPNG(outPNG)
}

func extractURLIcon(urlPath, outPNG string) error {
	data, err := os.ReadFile(urlPath)
	if err != nil {
		return writeMinimalPNG(outPNG)
	}
	lines := strings.Split(string(data), "\n")
	inSection := false
	var iconFile string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.EqualFold(line, "[InternetShortcut]")
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "iconfile=") {
			iconFile = strings.TrimSpace(line[len("IconFile="):])
			iconFile = normWinPath(iconFile)
			break
		}
	}
	if iconFile != "" {
		if st, err := os.Stat(iconFile); err == nil && !st.IsDir() {
			ext := strings.ToLower(filepath.Ext(iconFile))
			switch ext {
			case ".exe":
				if err := ExtractExeIcon(iconFile, outPNG); err == nil {
					return nil
				}
			case ".ico":
				if err := extractICOToPNG(iconFile, outPNG); err == nil {
					return nil
				}
			case ".dll":
				if err := extractIconExToPNG(iconFile, 0, outPNG); err == nil {
					return nil
				}
			}
		}
	}
	return writeMinimalPNG(outPNG)
}

func extractICOToPNG(icoPath, outPNG string) error {
	icoPath = filepath.Clean(icoPath)
	if st, err := os.Stat(icoPath); err != nil || st.IsDir() {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}
	b64In := base64.StdEncoding.EncodeToString([]byte(icoPath))
	b64Out := base64.StdEncoding.EncodeToString([]byte(outPNG))
	ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing
$icopath=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$outpath=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$icon = New-Object System.Drawing.Icon -ArgumentList $icopath
$bmp = $icon.ToBitmap()
$bmp.Save($outpath, [System.Drawing.Imaging.ImageFormat]::Png)
`, b64In, b64Out)
	return runDrawingIconPS(ps)
}

func extractIconExToPNG(filePath string, index int, outPNG string) error {
	filePath = filepath.Clean(filePath)
	if index < 0 {
		index = 0
	}
	if st, err := os.Stat(filePath); err != nil || st.IsDir() {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}
	b64File := base64.StdEncoding.EncodeToString([]byte(filePath))
	b64Out := base64.StdEncoding.EncodeToString([]byte(outPNG))
	ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class ShellIcons {
  [DllImport("shell32.dll", CharSet=CharSet.Unicode)]
  public static extern int ExtractIconEx(string lpszFile, int nIconIndex, out IntPtr phiconLarge, out IntPtr phiconSmall, int nIcons);
  [DllImport("user32.dll", SetLastError=true)]
  public static extern bool DestroyIcon(IntPtr hIcon);
}
"@
$fp=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$outpath=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$idx=%d
[IntPtr]$large=[IntPtr]::Zero
[IntPtr]$small=[IntPtr]::Zero
$n=[ShellIcons]::ExtractIconEx($fp, $idx, [ref]$large, [ref]$small, 1)
if ($n -le 0 -or $large -eq [IntPtr]::Zero) { exit 2 }
try {
  $icon = [System.Drawing.Icon]::FromHandle($large)
  $bmp = $icon.ToBitmap()
  $bmp.Save($outpath, [System.Drawing.Imaging.ImageFormat]::Png)
} finally {
  [void][ShellIcons]::DestroyIcon($large)
  if ($small -ne [IntPtr]::Zero) { [void][ShellIcons]::DestroyIcon($small) }
}
`, b64File, b64Out, index)
	return runDrawingIconPS(ps)
}

// 1x1 transparent PNG
const minimalPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func writeMinimalPNG(outPNG string) error {
	_ = os.MkdirAll(filepath.Dir(outPNG), 0o755)
	raw, err := base64.StdEncoding.DecodeString(minimalPNGBase64)
	if err != nil {
		return err
	}
	return os.WriteFile(outPNG, raw, 0o644)
}
