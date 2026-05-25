//go:build !windows

package ipc

import "errors"

// PipePath is unused on non-Windows builds.
const PipePath = ""

var errIPC = errors.New("ipc not supported on this platform")

// ForwardArgs is a no-op outside Windows (singleton mutex does not detect a running GUI).
func ForwardArgs(argv []string) error {
	return errIPC
}

// StartGUIServer is a no-op outside Windows.
func StartGUIServer(handler func(argv []string)) (func(), error) {
	return func() {}, nil
}
