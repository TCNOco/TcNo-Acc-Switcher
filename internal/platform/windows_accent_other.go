//go:build !windows

package platform

const WindowsAccentChangedEvent = "windows-accent-changed"

// CurrentWindowsAccentColor is empty on non-Windows platforms.
func CurrentWindowsAccentColor() string {
	return ""
}
