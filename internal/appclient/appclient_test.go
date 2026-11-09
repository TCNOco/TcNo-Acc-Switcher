package appclient

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"testing"
)

type testTransport struct{ calls, closed int }

func (t *testTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}
func (t *testTransport) CloseIdleConnections() { t.closed++ }

func TestOfflineTransportBlocksAndRecovers(t *testing.T) {
	defer SetOfflineMode(false)
	base := &testTransport{}
	transport := OfflineTransport(base)
	req, _ := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
	SetOfflineMode(true)
	if _, err := transport.RoundTrip(req); !errors.Is(err, ErrOfflineMode) || base.calls != 0 {
		t.Fatalf("offline request reached transport: calls=%d err=%v", base.calls, err)
	}
	SetOfflineMode(false)
	if _, err := transport.RoundTrip(req); err != nil || base.calls != 1 {
		t.Fatalf("online request did not recover: calls=%d err=%v", base.calls, err)
	}
	transport.(interface{ CloseIdleConnections() }).CloseIdleConnections()
	if base.closed != 1 {
		t.Fatal("offline wrapper did not release the underlying connection pool")
	}
}

func TestConnectivityErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{ErrOfflineMode, true}, {context.DeadlineExceeded, true},
		{&net.DNSError{Err: "no such host", Name: "example.invalid"}, true},
		{&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("network unreachable")}, true},
		{nil, false}, {context.Canceled, false}, {x509.UnknownAuthorityError{}, false},
		{&net.OpError{Op: "dial", Net: "tcp", Err: x509.UnknownAuthorityError{}}, false},
		{errors.New("HTTP 403"), false},
	} {
		if got := IsConnectivityError(tc.err); got != tc.want {
			t.Errorf("IsConnectivityError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
