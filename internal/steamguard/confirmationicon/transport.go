package confirmationicon

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"time"
)

const maxResponseHeaderBytes = 16 << 10

type ipResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type contextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type pinnedDialer struct {
	resolver ipResolver
	dialer   contextDialer
}

func secureTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	pinned := pinnedDialer{resolver: net.DefaultResolver, dialer: dialer}
	return &http.Transport{
		Proxy:                  nil,
		DialContext:            pinned.DialContext,
		ForceAttemptHTTP2:      true,
		DisableCompression:     true,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  5 * time.Second,
		ExpectContinueTimeout:  time.Second,
		IdleConnTimeout:        30 * time.Second,
		MaxIdleConns:           4,
		MaxIdleConnsPerHost:    2,
		MaxConnsPerHost:        2,
		MaxResponseHeaderBytes: maxResponseHeaderBytes,
		WriteBufferSize:        16 << 10,
		ReadBufferSize:         16 << 10,
	}
}

func (d pinnedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" || !validDNSName(host) || net.ParseIP(host) != nil {
		return nil, &Error{Kind: FailureBlocked}
	}
	addresses, err := d.resolver.LookupIPAddr(ctx, host)
	if ctx.Err() != nil {
		return nil, &Error{Kind: FailureCanceled}
	}
	if err != nil || len(addresses) == 0 {
		return nil, &Error{Kind: FailureUnavailable}
	}
	// Reject the whole answer if it contains any unsafe address. This prevents
	// fallback behavior from turning mixed public/private DNS into an SSRF path.
	for _, resolved := range addresses {
		if !publicAddress(resolved.IP) {
			return nil, &Error{Kind: FailureBlocked}
		}
	}
	for _, resolved := range addresses {
		connection, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		if ctx.Err() != nil {
			return nil, &Error{Kind: FailureCanceled}
		}
	}
	return nil, &Error{Kind: FailureUnavailable}
}

var deniedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func publicAddress(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsUnspecified() {
		return false
	}
	for _, network := range deniedNetworks {
		if network.Contains(address) {
			return false
		}
	}
	return true
}
