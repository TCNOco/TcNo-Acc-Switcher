//go:build windows

package winutil

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tc-hib/winres"
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
				if iconIdx != 0 {
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
	iconIdx := pickFirstIconGroup
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.EqualFold(line, "[InternetShortcut]")
			continue
		}
		if !inSection {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "iconfile=") {
			if eq := strings.IndexByte(line, '='); eq >= 0 {
				iconFile = normWinPath(strings.TrimSpace(line[eq+1:]))
			}
			continue
		}
		if strings.HasPrefix(lower, "iconindex=") {
			if eq := strings.IndexByte(line, '='); eq >= 0 {
				v := strings.TrimSpace(line[eq+1:])
				if n, err := strconv.Atoi(v); err == nil {
					iconIdx = n
				}
			}
		}
	}
	if iconFile != "" {
		if st, err := os.Stat(iconFile); err == nil && !st.IsDir() {
			ext := strings.ToLower(filepath.Ext(iconFile))
			switch ext {
			case ".exe", ".dll":
				if err := extractPEIconToPNG(iconFile, iconIdx, outPNG); err == nil {
					return nil
				}
			case ".ico":
				if err := extractICOToPNG(iconFile, outPNG); err == nil {
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
	f, err := os.Open(icoPath)
	if err != nil {
		return err
	}
	defer f.Close()
	icon, err := winres.LoadICO(f)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := icon.SaveICO(&buf); err != nil {
		return err
	}
	img, err := decodeBestFromICO(buf.Bytes())
	if err != nil {
		return err
	}
	return writePNG(outPNG, img)
}

func extractIconExToPNG(filePath string, index int, outPNG string) error {
	filePath = filepath.Clean(filePath)
	if st, err := os.Stat(filePath); err != nil || st.IsDir() {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}
	idx := index
	if index < -1 {
		// ExtractIconExW: negative values other than -1 load icon resource |-index|.
	} else if index < 0 {
		idx = pickFirstIconGroup
	}
	return extractPEIconToPNG(filePath, idx, outPNG)
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
