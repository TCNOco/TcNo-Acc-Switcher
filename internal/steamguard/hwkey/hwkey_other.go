//go:build !windows

package hwkey

// Security keys are Windows-only for now. The interface, the format and the
// vault paths are platform-neutral, so adding a CTAP-over-HID driver here later
// needs no change anywhere else.
func New() Authenticator {
	return Unsupported{Reason: "security keys are only supported on Windows in this build"}
}
