//go:build windows

package platform

import (
	"errors"
	"os"
	"syscall"
)

const (
	errnoSharingViolation = syscall.Errno(32)
	errnoLockViolation    = syscall.Errno(33)
)

func isSkippableCopyErrPlatform(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == errnoSharingViolation || errno == errnoLockViolation
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && errors.As(pathErr.Err, &errno) {
		return errno == errnoSharingViolation || errno == errnoLockViolation
	}
	return false
}
