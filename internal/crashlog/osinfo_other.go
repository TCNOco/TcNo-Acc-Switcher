//go:build !windows

package crashlog

import (
	"fmt"
	"runtime"
)

func osDisplayString() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
