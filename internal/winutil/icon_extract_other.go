//go:build !windows

package winutil

import "fmt"

// ExtractShortcutIcon is a no-op stub on non-Windows builds.
func ExtractShortcutIcon(shortcutPath, outPNG string) error {
	return fmt.Errorf("ExtractShortcutIcon: %w", ErrUnsupported)
}
