//go:build !windows

package winutil

// AttachParentConsole is a no-op outside Windows.
func AttachParentConsole() {}

// AllocConsole is a no-op outside Windows.
func AllocConsole() error { return nil }

// FreeConsole is a no-op outside Windows.
func FreeConsole() error { return nil }
