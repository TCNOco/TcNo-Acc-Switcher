package securemem

import (
	"bytes"
	"errors"
	"strconv"
	"testing"
)

func TestPlatformProtectorArbitraryLengths(t *testing.T) {
	for _, secret := range [][]byte{
		{0x5a},
		[]byte("enrollment"),
		bytes.Repeat([]byte{0x5a}, 15),
		bytes.Repeat([]byte{0x5a}, 16),
		bytes.Repeat([]byte{0x5a}, 17),
		[]byte(`{"serial":"EU-1234-5678-9012","deviceSecret":"fixture","pending":true}`),
	} {
		t.Run(strconv.Itoa(len(secret)), func(t *testing.T) {
			h, err := New().Store(secret)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := h.Destroy(); err != nil {
					t.Error(err)
				}
			})
			for range 2 {
				if err := h.With(func(got []byte) error {
					if !bytes.Equal(got, secret) {
						return errors.New("protected secret changed or included padding")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

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
