//go:build !windows

package winutil

// WaitForRestartParent is only needed for the Windows restart handoff.
func WaitForRestartParent(args []string) ([]string, error) {
	return args, nil
}
