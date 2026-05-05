//go:build !windows

package winutil

import "time"

func KillByName(names []string, method ClosingMethod, _ func() error) error {
	return ErrUnsupported
}

// WaitForegroundForExe is a Windows-only helper; stub always returns false.
func WaitForegroundForExe(_ string, _ time.Duration) bool {
	return false
}

func Start(exe string, args []string, opts StartOpts) error {
	return ErrUnsupported
}

func IsProcessElevated() bool {
	return false
}

func StartAsDesktopUser(exe string, args []string, opts StartOpts) error {
	return ErrUnsupported
}
