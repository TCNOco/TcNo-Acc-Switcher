//go:build windows

package crashlog

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

func osDisplayString() string {
	vi := windows.RtlGetVersion()
	if vi == nil {
		return fmt.Sprintf("Windows (? ? ?) %s/%s", runtime.GOOS, runtime.GOARCH)
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
	return fmt.Sprintf("%s (%d.%d.%d) %s/%s", label, maj, min, b, runtime.GOOS, runtime.GOARCH)
}
