// Package appclient holds shared outbound HTTP infrastructure for all platforms.
package appclient

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"
)

// ErrOfflineMode is returned by Shared when offline mode is enabled.
var ErrOfflineMode = errors.New("offline mode: outbound HTTP disabled")

var offline atomic.Bool

// SetOfflineMode toggles process-wide blocking of outbound HTTP on Shared.
func SetOfflineMode(on bool) {
	offline.Store(on)
}

// IsOfflineMode reports whether outbound HTTP is blocked.
func IsOfflineMode() bool {
	return offline.Load()
}

type offlineRoundTripper struct {
	base http.RoundTripper
}

func (t offlineRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if offline.Load() {
		return nil, ErrOfflineMode
	}
	if t.base == nil {
		t.base = http.DefaultTransport
	}
	return t.base.RoundTrip(req)
}

// Shared is the process-wide HTTP client for profile downloads, community/API
// requests, and other outbound calls. Platform packages (Steam, Discord, …)
// should use this instead of constructing separate clients.
var Shared = &http.Client{
	Timeout:   30 * time.Second,
	Transport: offlineRoundTripper{},
}
