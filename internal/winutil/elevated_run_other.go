//go:build !windows

package winutil

import (
	"errors"
	"time"
)

// ErrElevationDeclined is returned when the user dismisses the elevation prompt.
var ErrElevationDeclined = errors.New("elevation declined")

// RunSelfElevatedAndWait is Windows-only.
func RunSelfElevatedAndWait([]string, time.Duration) (int, error) {
	return 0, errors.New("elevated relaunch is only supported on Windows")
}
