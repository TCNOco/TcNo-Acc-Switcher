//go:build windows

package hwkey

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// The HRESULTs are spelled out rather than taken from the constants the mapping
// uses, so a wrong constant cannot make the test agree with itself.
func TestTranslateWindowsErrorMapsHRESULTs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"NTE_USER_CANCELLED", windows.Errno(0x80090036), ErrCancelled},
		{"ERROR_CANCELLED", windows.Errno(0x800704C7), ErrCancelled},
		{"NTE_NOT_FOUND", windows.Errno(0x80090011), ErrNoDevice},
		{"NTE_DEVICE_NOT_FOUND", windows.Errno(0x80090035), ErrNoDevice},
		{"ERROR_TIMEOUT", windows.Errno(0x800705B4), ErrNoDevice},
		{"wrapped", fmt.Errorf("get assertion: %w", windows.Errno(0x80090036)), ErrCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateWindowsError(tt.err)
			if !errors.Is(got, tt.want) {
				t.Fatalf("translateWindowsError(%v) = %v, want %v", tt.err, got, tt.want)
			}
			if !errors.Is(got, tt.err) {
				t.Fatalf("translateWindowsError(%v) dropped the original error", tt.err)
			}
		})
	}
}

func TestTranslateWindowsErrorFallsBackToText(t *testing.T) {
	// Errors raised before webauthn.dll is reached carry no HRESULT.
	if got := translateWindowsError(errors.New("the operation was cancelled")); !errors.Is(got, ErrCancelled) {
		t.Fatalf("got %v, want ErrCancelled", got)
	}
	if got := translateWindowsError(errors.New("disk on fire")); errors.Is(got, ErrCancelled) || errors.Is(got, ErrNoDevice) {
		t.Fatalf("got %v, want the error unchanged", got)
	}
}

func TestKeyTimeoutFollowsContextDeadline(t *testing.T) {
	if got := keyTimeout(context.Background()); got != defaultKeyTimeout {
		t.Fatalf("no deadline: got %v, want %v", got, defaultKeyTimeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if got := keyTimeout(ctx); got <= 25*time.Second || got > 30*time.Second {
		t.Fatalf("30s deadline: got %v, want just under 30s", got)
	}

	// Windows reads zero milliseconds as "no timeout", so an expired deadline
	// must still round up to something it will act on.
	expired, cancelExpired := context.WithTimeout(context.Background(), -time.Second)
	defer cancelExpired()
	if got := keyTimeout(expired); got.Milliseconds() <= 0 {
		t.Fatalf("expired deadline: got %v, want a positive timeout", got)
	}
}
