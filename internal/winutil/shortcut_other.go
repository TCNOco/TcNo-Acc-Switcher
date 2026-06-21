//go:build !windows

package winutil

import "fmt"

func ReadLnkShortcut(lnkPath string) (target string, arguments string, iconLocation string, err error) {
	return "", "", "", fmt.Errorf("shortcuts: %w", ErrUnsupported)
}

// WriteShortcutLnk is unsupported outside Windows.
func WriteShortcutLnk(shortcutPath, targetExe, arguments, workingDir, description, iconLocation, appUserModelID string) error {
	return fmt.Errorf("shortcuts: %w", ErrUnsupported)
}
