//go:build !windows

package winutil

import "fmt"

var singletonReleaser func()

// RegisterSingletonReleaser is a no-op on non-Windows.
func RegisterSingletonReleaser(f func()) {
	singletonReleaser = f
}

// RestartElevated is unsupported on non-Windows.
func RestartElevated(extraArgs []string) error {
	return fmt.Errorf("restart elevated: %w", ErrUnsupported)
}
