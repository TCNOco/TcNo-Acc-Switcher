package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
)

// Explicitly requested, local diagnostics only. No profiling server is started.
func writeGoroutineLeakProfile(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("goroutine leak report requires an absolute filename")
	}
	profile := pprof.Lookup("goroutineleak")
	if profile == nil {
		return fmt.Errorf("goroutine leak profiling is unavailable")
	}
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	// WriteTo performs the leak-detection GC. Debug 1 includes only leaks;
	// debug 2 would include all goroutines, including healthy background work.
	err = profile.WriteTo(output, 1)
	return errors.Join(err, output.Close())
}
