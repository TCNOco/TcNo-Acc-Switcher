//go:build windows

package winutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HomeDesktopShortcutName is the fixed .lnk basename for the app home shortcut.
const HomeDesktopShortcutName = "TcNo Account Switcher.lnk"

// desktopShortcutSearchDirs returns common user Desktop locations (includes OneDrive Desktop).
func desktopShortcutSearchDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(filepath.Join(home, "Desktop"))
	add(filepath.Join(home, "OneDrive", "Desktop"))
	return out
}

// HomeDesktopShortcutExists reports whether our home shortcut exists on any searched Desktop.
func HomeDesktopShortcutExists() bool {
	name := HomeDesktopShortcutName
	for _, dir := range desktopShortcutSearchDirs() {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// homeDesktopShortcutWriteDir picks the first existing Desktop directory, else %UserProfile%\Desktop.
func homeDesktopShortcutWriteDir() (string, error) {
	for _, dir := range desktopShortcutSearchDirs() {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("user home unknown")
	}
	return filepath.Join(home, "Desktop"), nil
}

// SetHomeDesktopShortcut creates or removes the home shortcut on the user's Desktop.
func SetHomeDesktopShortcut(create bool) error {
	if !create {
		name := HomeDesktopShortcutName
		for _, dir := range desktopShortcutSearchDirs() {
			_ = os.Remove(filepath.Join(dir, name))
		}
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)
	dir, err := homeDesktopShortcutWriteDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out := filepath.Join(dir, HomeDesktopShortcutName)
	workDir := filepath.Dir(self)
	desc := "TcNo Account Switcher — Home"
	icon := self + ",0"
	return WriteShortcutLnk(out, self, "", workDir, desc, icon)
}
