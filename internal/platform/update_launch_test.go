package platform

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsTransientNetworkError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("updater: all providers failed: github: dial tcp: lookup api.github.com: no such host"), true},
		{errors.New("Get \"https://api.github.com\": dial tcp: i/o timeout"), true},
		{errors.New("connection refused"), true},
		{errors.New("temporary failure in name resolution"), true},
		{errors.New("HTTP 404 Not Found"), false},
		{errors.New("signature verification failed"), false},
	}
	for _, tc := range cases {
		tc := tc
		name := "nil"
		if tc.err != nil {
			name = tc.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := isTransientNetworkError(tc.err); got != tc.want {
				t.Fatalf("isTransientNetworkError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsTransientNetworkError_Wrapped(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("wrapped: %w", errors.New("lookup api.github.com: no such host"))
	if !isTransientNetworkError(err) {
		t.Fatal("expected wrapped DNS error to be transient")
	}
}
