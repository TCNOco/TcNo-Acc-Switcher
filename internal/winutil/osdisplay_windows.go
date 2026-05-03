//go:build windows

package winutil

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// OSDisplayString returns a human-readable OS line for statistics (Windows build from RtlGetVersion).
func OSDisplayString() string {
	vi := windows.RtlGetVersion()
	if vi == nil {
		return fmt.Sprintf("Windows (? ? ?) %s", archDisplay())
	}
	maj, min, b := vi.MajorVersion, vi.MinorVersion, vi.BuildNumber
	label := "Windows"
	if maj == 10 {
		if b >= 22000 {
			label = "Windows 11"
		} else {
			label = "Windows 10"
		}
	}
	return fmt.Sprintf("%s (%d.%d.%d) %s", label, maj, min, b, archDisplay())
}
