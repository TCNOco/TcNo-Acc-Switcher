//go:build windows

package ipc

import (
	"errors"
	"io"
	"net"
	"testing"
)

func TestIsClosedPipeErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"net.ErrClosed", net.ErrClosed, true},
		{"io.EOF", io.EOF, true},
		{"wrapped net.ErrClosed", &net.OpError{Op: "accept", Err: net.ErrClosed}, true},
		{"wrapped io.EOF", &net.OpError{Op: "accept", Err: io.EOF}, true},
		{"other error", errors.New("boom"), false},
		{"nil inside OpError", &net.OpError{Op: "accept"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isClosedPipeErr(tc.err); got != tc.want {
				t.Errorf("isClosedPipeErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestStopIsIdempotent(t *testing.T) {
	stop, err := StartGUIServer(func(argv []string) {})
	if err != nil {
		// Pipe might be in use by another test; skip.
		t.Skipf("StartGUIServer: %v", err)
	}
	stop()
	stop() // must not panic on double close
}
