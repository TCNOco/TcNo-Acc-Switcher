//go:build windows

package winutil

import (
	"os"
	"path/filepath"
)

// HomeDesktopShortcutName is the fixed .lnk basename for the app home shortcut.
const HomeDesktopShortcutName = "TcNo Account Switcher.lnk"

// HomeDesktopShortcutExists reports whether our home shortcut exists on any searched Desktop.
func HomeDesktopShortcutExists() bool {
	name := HomeDesktopShortcutName
	for _, dir := range DesktopSearchDirs() {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// SetHomeDesktopShortcut creates or removes the home shortcut on the user's Desktop.
func SetHomeDesktopShortcut(create bool) error {
	if !create {
		name := HomeDesktopShortcutName
		for _, dir := range DesktopSearchDirs() {
			_ = os.Remove(filepath.Join(dir, name))
		}
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)
	dir, err := DesktopWriteDir()
	if err != nil {
		return err
	}
	out := filepath.Join(dir, HomeDesktopShortcutName)
	workDir := filepath.Dir(self)
	desc := "TcNo Account Switcher — Home"
	icon := self + ",0"
	return WriteShortcutLnk(out, self, "", workDir, desc, icon, ShortcutAppUserModelID("home"))
}
