package steambrowser

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchCertificateRejectsNonHTTPS(t *testing.T) {
	for _, raw := range []string{
		"http://steamcommunity.com/",
		"about:blank",
		"file:///C:/Windows/System32/drivers/etc/hosts",
		"https://",
	} {
		if _, err := FetchCertificate(context.Background(), raw); !errors.Is(err, ErrNoCertificate) {
			t.Errorf("FetchCertificate(%q) error = %v, want ErrNoCertificate", raw, err)
		}
	}
}

// FetchCertificate reads the live certificate, so it is exercised against a local
// TLS server rather than the internet.
func TestFetchCertificateReadsTheServedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()

	// httptest signs with its own throwaway CA, which the system store does not
	// know. Trusting it here keeps verification on for the code under test.
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	original := defaultRootCAs
	defaultRootCAs = pool
	defer func() { defaultRootCAs = original }()

	cert, err := FetchCertificate(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchCertificate: %v", err)
	}
	if cert.SHA256 == "" {
		t.Error("no certificate fingerprint")
	}
	if cert.NotAfter.Before(time.Now()) {
		t.Errorf("NotAfter %v is already past", cert.NotAfter)
	}
	if cert.TLSVersion == "" || cert.CipherSuite == "" {
		t.Errorf("missing negotiated details: version %q cipher %q", cert.TLSVersion, cert.CipherSuite)
	}
	// The popover must not imply this came from the page's own connection.
	if !cert.FromSeparateConnection {
		t.Error("FromSeparateConnection = false, want true")
	}
}

func TestFetchCertificateRejectsAnUntrustedChain(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()

	// No pool override: the throwaway CA is unknown, so verification must fail
	// rather than silently reporting an unverified certificate.
	if _, err := FetchCertificate(context.Background(), server.URL); err == nil {
		t.Error("got nil error for an untrusted chain, want a verification failure")
	}
}

func TestFetchCertificateHonoursContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FetchCertificate(ctx, "https://steamcommunity.com/"); err == nil {
		t.Error("got nil error for a cancelled context")
	}
}
