//go:build darwin

package platform

import pos "TcNo-Acc-Switcher/internal/platform/os/darwin"

func findExeViaStartMenuShortcuts(entry platformEntry, exeName string) (string, bool) {
	return pos.FindExeViaShortcuts(entry.GetPathFromShortcutNamed, exeName)
}
