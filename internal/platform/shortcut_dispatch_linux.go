//go:build linux

package platform

import pos "TcNo-Acc-Switcher/internal/platform/os/linux"

func findExeViaStartMenuShortcuts(entry PlatformEntry, exeName string) (string, bool) {
	return pos.FindExeViaShortcuts(entry.GetPathFromShortcutNamed, exeName)
}
