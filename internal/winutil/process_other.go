//go:build !windows && !unix

package winutil

import "time"

// Neither Windows nor a Unix: no process table this package knows how to read.
// Stubs, so the package still builds for a target the app does not ship to.

func KillByName(names []string, method ClosingMethod, _ func() error) error {
	return ErrUnsupported
}

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

func SnapshotRunningExeBasenames() (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}

func IsExeRunning(_ string) bool {
	return false
}

func SnapshotMatchingPIDs(_ map[string]struct{}) (map[uint32]string, error) {
	return nil, nil
}
