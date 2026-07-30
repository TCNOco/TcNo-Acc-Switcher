//go:build windows

package fsutil

import (
	"errors"
	"syscall"
)

const (
	errnoAccessDenied     = syscall.Errno(5)
	errnoSharingViolation = syscall.Errno(32)
	errnoLockViolation    = syscall.Errno(33)
)

// isRetriableRenameErr reports whether a failed rename is one another process
// is expected to stop causing on its own, rather than a real error.
func isRetriableRenameErr(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errnoAccessDenied || errno == errnoSharingViolation || errno == errnoLockViolation
}
