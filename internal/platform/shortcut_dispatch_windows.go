//go:build windows

package platform

import winos "TcNo-Acc-Switcher/internal/platform/os/windows"

func findExeViaStartMenuShortcuts(entry platformEntry, exeName string) (string, bool) {
	return winos.FindExeViaShortcuts(entry.GetPathFromShortcutNamed, exeName)
}
