//go:build windows

package winutil

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// WrapIfElevationRequired maps Windows ERROR_ELEVATION_REQUIRED (740) to [NewNeedsAdminError]
// so the frontend can offer restarting elevated instead of surfacing a raw fork/exec message.
func WrapIfElevationRequired(err error) error {
	if err == nil {
		return nil
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == windows.ERROR_ELEVATION_REQUIRED {
		return NewNeedsAdminError("")
	}
	return err
}
