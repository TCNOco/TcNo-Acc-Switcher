package app

import (
	"runtime"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// ShouldElevateAtStartup reports whether this launch should hand over to an
// elevated copy of itself. Console commands are left alone: a UAC prompt in the
// middle of a scripted swap, on a process whose output the caller is reading,
// helps nobody.
func ShouldElevateAtStartup(parsed cli.Parsed, settings platform.AppSettings) bool {
	if runtime.GOOS != "windows" || !settings.AlwaysRunAsAdmin {
		return false
	}
	if parsed.NeedsHeadlessMutex() || parsed.IsListCommand() {
		return false
	}
	return !winutil.IsProcessElevated()
}
