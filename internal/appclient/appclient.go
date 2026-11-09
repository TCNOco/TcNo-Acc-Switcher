// Package appclient holds shared outbound HTTP infrastructure for all platforms.
package appclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
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

// IsConnectivityError identifies an unreachable network without treating HTTP
// responses or certificate validation failures as proof the machine is offline.
func IsConnectivityError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var certificate *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &certificate) || errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &invalid) {
		return false
	}
	if errors.Is(err, ErrOfflineMode) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var dns *net.DNSError
	var op *net.OpError
	if errors.As(err, &dns) || errors.As(err, &op) {
		return true
	}
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

// OfflineTransport preserves a caller's transport policy while applying the
// process-wide offline setting at the point each request would leave the app.
func OfflineTransport(base http.RoundTripper) http.RoundTripper {
	return offlineRoundTripper{base: base}
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

func (t offlineRoundTripper) CloseIdleConnections() {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if closer, ok := base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// Shared is the process-wide HTTP client for profile downloads, community/API
// requests, and other outbound calls. Platform packages (Steam, Discord, …)
// should use this instead of constructing separate clients.
var Shared = &http.Client{
	Timeout:   30 * time.Second,
	Transport: offlineRoundTripper{},
}
