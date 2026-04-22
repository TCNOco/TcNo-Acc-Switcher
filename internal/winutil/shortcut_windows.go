//go:build windows

package winutil

import (
	"fmt"
	"os"
)

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
