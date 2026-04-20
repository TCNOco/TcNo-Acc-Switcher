//go:build windows

package platform

import winos "TcNo-Acc-Switcher/internal/platform/os/windows"

func findExeViaStartMenuShortcuts(entry PlatformEntry, exeName string) (string, bool) {
	return winos.FindExeViaShortcuts(entry.GetPathFromShortcutNamed, exeName)
}
