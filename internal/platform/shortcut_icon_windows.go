//go:build windows

package platform

import (
	"strings"

	winos "TcNo-Acc-Switcher/internal/platform/os/windows"
)

func findStartMenuIconShortcut(entry PlatformEntry) (string, bool) {
	shortcuts := strings.Split(strings.TrimSpace(entry.GetPathFromShortcutNamed), "|")
	for _, raw := range shortcuts {
		title := strings.TrimSpace(raw)
		if title == "" {
			continue
		}
		paths := winos.OrderedLnkPathsForTitle(title)
		if len(paths) == 0 {
			continue
		}
		return paths[0], true
	}
	return "", false
}
