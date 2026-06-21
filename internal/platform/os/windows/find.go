package windows

import (
	"os"
	"path/filepath"
	"strings"
)

// FindExeViaShortcuts searches Start Menu / Desktop shortcuts (pipe-separated names in shortcutNamed).
func FindExeViaShortcuts(shortcutNamed, exeName string) (string, bool) {
	if exeName == "" || strings.TrimSpace(shortcutNamed) == "" {
		return "", false
	}
	titles := strings.Split(shortcutNamed, "|")
	for _, raw := range titles {
		title := strings.TrimSpace(raw)
		if title == "" {
			continue
		}
		for _, lnk := range OrderedLnkPathsForTitle(title) {
			target, err := ResolveLnkTarget(lnk)
			if err != nil || target == "" {
				continue
			}
			dir := filepath.Dir(target)
			candidate := filepath.Join(dir, exeName)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return filepath.Clean(candidate), true
			}
		}
	}
	return "", false
}
