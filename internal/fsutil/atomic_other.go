//go:build !windows

package fsutil

// Rename on POSIX does not fail on open handles, so there is nothing to wait out.
func isRetriableRenameErr(error) bool { return false }
