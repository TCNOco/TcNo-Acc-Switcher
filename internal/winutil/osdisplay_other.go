//go:build !windows

package winutil

import (
	"fmt"
	"runtime"
)

// OSDisplayString returns a simple OS / architecture line on non-Windows builds.
func OSDisplayString() string {
	return fmt.Sprintf("%s %s", runtime.GOOS, archDisplay())
}
