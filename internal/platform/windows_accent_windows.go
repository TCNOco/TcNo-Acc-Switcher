//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const WindowsAccentChangedEvent = "windows-accent-changed"

// CurrentWindowsAccentColor returns the current Windows accent colour as #rrggbb.
func CurrentWindowsAccentColor() string {
	if color, ok := readAccentDWORD(`Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, "AccentColorMenu"); ok {
		return color
	}
	if color, ok := readAccentDWORD(`Software\Microsoft\Windows\DWM`, "AccentColor"); ok {
		return color
	}
	if color, ok := readAccentDWORD(`Software\Microsoft\Windows\DWM`, "ColorizationColor"); ok {
		return color
	}
	return ""
}

func readAccentDWORD(path, name string) (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()

	value, _, err := key.GetIntegerValue(name)
	if err != nil {
		return "", false
	}
	return abgrDWORDToHex(value), true
}

func abgrDWORDToHex(value uint64) string {
	r := byte(value & 0xff)
	g := byte((value >> 8) & 0xff)
	b := byte((value >> 16) & 0xff)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
