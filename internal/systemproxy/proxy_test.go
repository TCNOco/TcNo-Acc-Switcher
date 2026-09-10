package systemproxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestWindowsProxyMap(t *testing.T) {
	for _, tc := range []struct {
		name, target, server, bypass, want string
		bad                                bool
	}{
		{"v2ray local proxy", "https://api.steampowered.com/auth", "127.0.0.1:10809", "", "http://127.0.0.1:10809", false},
		{"secure means CONNECT over HTTP", "https://api.steampowered.com", "http=localhost:80;https=localhost:10809", "", "http://localhost:10809", false},
		{"http mapping", "http://example.com", "http=localhost:80;https=localhost:81", "", "http://localhost:80", false},
		{"socks fallback", "https://example.com", "socks=127.0.0.1:10808", "", "socks5://127.0.0.1:10808", false},
		{"explicit scheme", "https://example.com", "https=https://proxy.example:443", "", "https://proxy.example:443", false},
		{"disabled", "https://example.com", "", "", "", false},
		{"other scheme only", "https://example.com", "http=localhost:80", "", "", false},
		{"bypass wildcard", "https://api.steampowered.com", "localhost:80", "*.steampowered.com", "", false},
		{"bypass exact case insensitive", "https://api.steampowered.com", "localhost:80", "API.STEAMPOWERED.COM", "", false},
		{"bypass not substring", "https://api.steampowered.com.evil", "localhost:80", "*.steampowered.com", "http://localhost:80", false},
		{"local host", "https://intranet", "localhost:80", "<local>", "", false},
		{"local excludes public", "https://example.com", "localhost:80", "<local>", "http://localhost:80", false},
		{"loopback", "http://127.0.0.1:1234", "localhost:80", "", "", false},
		{"IPv6 proxy", "https://example.com", "[::1]:10809", "", "http://[::1]:10809", false},
		{"bypass port", "https://example.com:443", "localhost:80", "example.com:443", "", false},
		{"empty mapping fails closed", "https://example.com", "https=", "", "", true},
		{"invalid proxy fails closed", "https://example.com", "http://", "", "", true},
		{"invalid port", "https://example.com", "localhost:abc", "", "", true},
		{"unsupported protocol", "https://example.com", "ftp://localhost:80", "", "", true},
		{"no credential parsing", "https://example.com", "http://user:password@localhost:80", "", "", true},
		{"first PAC proxy", "https://example.com", "first.example:80;second.example:80", "", "http://first.example:80", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, err := url.Parse(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			got, err := proxyForConfig(target, tc.server, tc.bypass)
			if tc.bad {
				if err == nil {
					t.Fatal("invalid configuration allowed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			value := ""
			if got != nil {
				value = got.String()
			}
			if value != tc.want {
				t.Fatalf("proxy = %q, want %q", value, tc.want)
			}
		})
	}
}

func TestHTTPSUsesProxyCONNECTWithoutDirectFallback(t *testing.T) {
	var calls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != "api.steampowered.com:443" {
			t.Errorf("unexpected proxy request: %s %s", r.Method, r.Host)
		}
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	var direct atomic.Int32
	dialer := &net.Dialer{Timeout: time.Second}
	proxyURL, _ := url.Parse(proxy.URL)
	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) { return proxyForConfig(req.URL, proxyURL.Host, "") },
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != proxyURL.Host {
				direct.Add(1)
				return nil, errors.New("direct connection forbidden")
			}
			return dialer.DialContext(ctx, network, address)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	_, err := client.Get("https://api.steampowered.com/auth")
	if err == nil || calls.Load() != 1 || direct.Load() != 0 {
		t.Fatalf("err=%v, CONNECT=%d, direct=%d", err, calls.Load(), direct.Load())
	}
}
