package steambrowser

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ErrNoCertificate reports a URL that has no TLS to describe.
var ErrNoCertificate = errors.New("steambrowser: page is not served over https")

const certDialTimeout = 8 * time.Second

// defaultRootCAs is nil in production, which means the system trust store. Tests
// substitute a pool so verification can stay switched on against a throwaway CA
// rather than being disabled to make them pass.
var defaultRootCAs *x509.CertPool

// Certificate is what the lock icon's popover shows.
//
// FromSeparateConnection is always true and is carried into the UI deliberately:
// this comes from a fresh handshake made by the application, not from the
// connection the page was actually loaded over. In practice they match, but the
// popover says so rather than implying it is the page's own certificate.
type Certificate struct {
	Host                   string    `json:"host"`
	Subject                string    `json:"subject"`
	Issuer                 string    `json:"issuer"`
	NotBefore              time.Time `json:"notBefore"`
	NotAfter               time.Time `json:"notAfter"`
	DNSNames               []string  `json:"dnsNames"`
	SerialNumber           string    `json:"serialNumber"`
	SHA256                 string    `json:"sha256"`
	TLSVersion             string    `json:"tlsVersion"`
	CipherSuite            string    `json:"cipherSuite"`
	FromSeparateConnection bool      `json:"fromSeparateConnection"`
}

// FetchCertificate describes the certificate serving rawURL's host.
//
// Verification is left on. A page the window managed to load has already passed
// the webview's own check, so a failure here is worth surfacing rather than
// papering over with InsecureSkipVerify.
func FetchCertificate(ctx context.Context, rawURL string) (Certificate, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Certificate{}, fmt.Errorf("steambrowser: parse url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return Certificate{}, ErrNoCertificate
	}
	host := canonicalHost(parsed.Hostname())
	if host == "" {
		return Certificate{}, ErrNoCertificate
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: certDialTimeout},
		Config: &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
			RootCAs:    defaultRootCAs,
		},
	}
	dialCtx, cancel := context.WithTimeout(ctx, certDialTimeout)
	defer cancel()

	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return Certificate{}, fmt.Errorf("steambrowser: tls handshake with %s: %w", host, err)
	}
	defer conn.Close()

	state := conn.(*tls.Conn).ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return Certificate{}, fmt.Errorf("steambrowser: %s presented no certificate", host)
	}
	leaf := state.PeerCertificates[0]
	digest := sha256.Sum256(leaf.Raw)

	return Certificate{
		Host:                   host,
		Subject:                leaf.Subject.CommonName,
		Issuer:                 leaf.Issuer.CommonName,
		NotBefore:              leaf.NotBefore,
		NotAfter:               leaf.NotAfter,
		DNSNames:               leaf.DNSNames,
		SerialNumber:           leaf.SerialNumber.String(),
		SHA256:                 hex.EncodeToString(digest[:]),
		TLSVersion:             tls.VersionName(state.Version),
		CipherSuite:            tls.CipherSuiteName(state.CipherSuite),
		FromSeparateConnection: true,
	}, nil
}
