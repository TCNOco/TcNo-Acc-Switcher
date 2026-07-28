package confirmationicon

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

type resolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f resolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func TestPinnedDialerRejectsPrivateAndMixedDNSAnswers(t *testing.T) {
	tests := []struct {
		name      string
		addresses []net.IPAddr
	}{
		{name: "loopback", addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}},
		{name: "private", addresses: []net.IPAddr{{IP: net.ParseIP("10.0.0.4")}}},
		{name: "metadata", addresses: []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}},
		{name: "ipv6-private", addresses: []net.IPAddr{{IP: net.ParseIP("fd00::1")}}},
		{name: "documentation", addresses: []net.IPAddr{{IP: net.ParseIP("203.0.113.8")}}},
		{name: "mixed", addresses: []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("192.168.1.1")}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			dialer := pinnedDialer{
				resolver: resolverFunc(func(context.Context, string) ([]net.IPAddr, error) { return test.addresses, nil }),
				dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
					called = true
					return nil, errors.New("must not dial")
				}),
			}
			_, err := dialer.DialContext(context.Background(), "tcp", "cdn.example.com:443")
			var safe *Error
			if !errors.As(err, &safe) || safe.Kind != FailureBlocked {
				t.Fatalf("got %v", err)
			}
			if called {
				t.Fatal("unsafe DNS answer reached dialer")
			}
		})
	}
}

func TestPinnedDialerDialsVerifiedIPAddressNotHostname(t *testing.T) {
	var dialed string
	dialer := pinnedDialer{
		resolver: resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host != allowedHost {
				t.Fatalf("resolved unexpected host %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
		}),
		dialer: dialerFunc(func(_ context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" {
				t.Fatalf("network = %q", network)
			}
			dialed = address
			return nil, errors.New("private low-level dial error")
		}),
	}
	_, err := dialer.DialContext(context.Background(), "tcp", allowedHost+":443")
	var safe *Error
	if !errors.As(err, &safe) || safe.Kind != FailureUnavailable {
		t.Fatalf("got %v", err)
	}
	if dialed != "8.8.8.8:443" {
		t.Fatalf("dialed %q instead of pinned address", dialed)
	}
	if strings.Contains(err.Error(), "low-level") || strings.Contains(err.Error(), allowedHost) {
		t.Fatalf("error disclosed connection details: %q", err)
	}
}

func TestPinnedDialerRejectsUnexpectedPortAndLiteralHost(t *testing.T) {
	dialer := pinnedDialer{
		resolver: resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			t.Fatal("resolver must not be called")
			return nil, nil
		}),
		dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("dialer must not be called")
			return nil, nil
		}),
	}
	for _, address := range []string{"cdn.example.com:80", "127.0.0.1:443", "[::1]:443"} {
		if _, err := dialer.DialContext(context.Background(), "tcp", address); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
}

func TestPublicAddressClassification(t *testing.T) {
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicAddress(net.ParseIP(raw)) {
			t.Fatalf("public address %s rejected", raw)
		}
	}
	for _, raw := range []string{"0.0.0.0", "127.0.0.1", "10.1.2.3", "100.64.0.1", "169.254.1.1", "192.0.2.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "::", "::1", "fd00::1", "fe80::1", "2001:db8::1"} {
		if publicAddress(net.ParseIP(raw)) {
			t.Fatalf("unsafe address %s accepted", raw)
		}
	}
}
