//go:build !windows

package winutil

// IsProtocolRegistered is always false outside Windows.
func IsProtocolRegistered() bool { return false }

// RegisterProtocol is a no-op outside Windows.
func RegisterProtocol(exePath string) error { return nil }

// UnregisterProtocol is a no-op outside Windows.
func UnregisterProtocol() error { return nil }
