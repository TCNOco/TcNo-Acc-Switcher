//go:build !windows

package winutil

func KillByName(names []string, method ClosingMethod) error {
	return ErrUnsupported
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
