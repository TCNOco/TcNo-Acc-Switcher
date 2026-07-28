// Package securemem keeps short-lived secrets behind an OS-specific memory
// protection boundary and guarantees zeroing when a handle is destroyed.
package securemem

import "errors"

var ErrUnavailable = errors.New("secure memory is unavailable")

// Handle owns a protected secret. With exposes a disposable copy only for the
// duration of fn; callers must not retain the slice.
type Handle interface {
	With(fn func([]byte) error) error
	Destroy() error
}

// Protector moves a secret into protected storage.
type Protector interface {
	Store(secret []byte) (Handle, error)
}

// New returns the platform protector.
func New() Protector { return newPlatformProtector() }

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
