//go:build !windows

package winutil

import "fmt"

const HomeDesktopShortcutName = "TcNo Account Switcher.lnk"

func HomeDesktopShortcutExists() bool { return false }

func SetHomeDesktopShortcut(create bool) error {
	if create {
		return fmt.Errorf("desktop home shortcut is only supported on Windows")
	}
	return nil
}
