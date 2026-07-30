package fsutil

import (
	"os"
	"path/filepath"
	"time"

	"TcNo-Acc-Switcher/internal/actionlog"
)

const (
	renameAttempts   = 5
	renameBackoffMin = 10 * time.Millisecond
	renameBackoffMax = 100 * time.Millisecond
)

// WriteFileAtomic writes data to path using a temp file in the same directory.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".atomic-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if perm != 0 {
		if err := os.Chmod(tmpPath, perm); err != nil {
			cleanup()
			return err
		}
	}
	if err := renameWithRetry(tmpPath, path, os.Rename); err != nil {
		cleanup()
		actionlog.Record("file:write", path, "", err)
		return err
	}
	actionlog.Record("file:write", path, "", nil)
	return nil
}

// renameWithRetry replaces newPath with oldPath, retrying a bounded number of
// times while the destination is held by another process. An AV scanner, search
// indexer or cloud-sync agent opening the file without FILE_SHARE_DELETE makes
// MoveFileEx fail for a few hundred milliseconds, which would otherwise fail an
// entire settings save.
//
// The rename function is a parameter so tests can drive the loop with synthetic
// failures without OS-specific setup. Production callers pass os.Rename.
func renameWithRetry(oldPath, newPath string, rename func(oldPath, newPath string) error) error {
	backoff := renameBackoffMin
	var err error
	for attempt := 0; attempt < renameAttempts; attempt++ {
		if err = rename(oldPath, newPath); err == nil {
			return nil
		}
		if !isRetriableRenameErr(err) {
			return err
		}
		if attempt == renameAttempts-1 {
			break
		}
		time.Sleep(backoff)
		if backoff < renameBackoffMax {
			backoff *= 2
		}
	}
	return err
}
