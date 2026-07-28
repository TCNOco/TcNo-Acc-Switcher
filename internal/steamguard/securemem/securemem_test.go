package securemem

import (
	"bytes"
	"errors"
	"testing"
)

func TestPlatformProtectorLifecycle(t *testing.T) {
	secret := bytes.Repeat([]byte{0x5a}, 32)
	h, err := New().Store(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.With(func(got []byte) error {
		if !bytes.Equal(got, secret) {
			return errors.New("secure-memory round trip changed the secret")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := h.With(func([]byte) error { return nil }); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("use after destroy: %v", err)
	}
}
