package fsutil

import (
	"errors"
	"os"
	"time"

	"TcNo-Acc-Switcher/internal/actionlog"
)

// RemoveAllWithRetry calls os.RemoveAll with bounded exponential-backoff
// retry. It returns nil if the path is gone (whether because it never
// existed or because a retry succeeded) and the last observed error
// otherwise.
//
// The remove function is taken as a parameter so tests can drive the retry
// loop with synthetic failures without OS-specific setup. Production
// callers should pass os.RemoveAll.
func RemoveAllWithRetry(path string, total time.Duration, remove func(string) error) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(total)
	backoff := 100 * time.Millisecond
	var lastErr error
	for {
		if err := remove(path); err == nil {
			actionlog.Record("file:delete", path, "", nil)
			return nil
		} else {
			lastErr = err
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(backoff)
		if backoff < 500*time.Millisecond {
			backoff *= 2
		}
	}
	actionlog.Record("file:delete", path, "", lastErr)
	return lastErr
}
