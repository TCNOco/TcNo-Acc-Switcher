//go:build !windows

package qrregion

import (
	"context"
	"runtime"
)

type unsupportedBackend struct{}

func newBackend() backend { return unsupportedBackend{} }

func (unsupportedBackend) selectRegion(context.Context) (Rect, error) {
	return Rect{}, &UnsupportedError{GOOS: runtime.GOOS}
}

func (unsupportedBackend) captureRegion(Rect) (Frame, error) {
	return Frame{}, &UnsupportedError{GOOS: runtime.GOOS}
}
