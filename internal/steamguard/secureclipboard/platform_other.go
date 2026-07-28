//go:build !windows

package secureclipboard

import "runtime"

type unsupportedPlatform struct{}

func newPlatform() clipboardPlatform { return unsupportedPlatform{} }

func (unsupportedPlatform) write(code) (writeStamp, error) {
	return writeStamp{}, &UnsupportedError{GOOS: runtime.GOOS}
}

func (unsupportedPlatform) clearIfUnchanged(writeStamp) (bool, error) {
	return false, &UnsupportedError{GOOS: runtime.GOOS}
}
