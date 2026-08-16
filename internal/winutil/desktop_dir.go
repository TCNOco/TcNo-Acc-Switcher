package winutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DesktopSearchDirs returns the user's candidate Desktop directories, most likely first.
// OneDrive's Known Folder Move relocates the Desktop to %UserProfile%\OneDrive\Desktop and removes
// the original, so both have to be considered on any given machine.
func DesktopSearchDirs() []string {
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

// DesktopWriteDir returns the first Desktop directory that already exists on disk.
// It never falls back to a path it would have to create: once Known Folder Move has taken
// %UserProfile%\Desktop away, re-creating it produces a folder no shell surface shows, so writing a
// shortcut there succeeds while the user sees nothing.
func DesktopWriteDir() (string, error) {
	for _, dir := range DesktopSearchDirs() {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no Desktop folder found for the current user")
}
