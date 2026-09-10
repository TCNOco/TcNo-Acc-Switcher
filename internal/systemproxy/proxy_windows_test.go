package systemproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestNativePACResolution(t *testing.T) {
	// Serve only a PAC script locally. No OS proxy settings are changed and no
	// request is sent to the target host.
	pac := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		_, _ = w.Write([]byte("function FindProxyForURL(url, host) { if (host == 'api.steampowered.com') return 'PROXY 127.0.0.1:10809'; return 'DIRECT'; }"))
	}))
	defer pac.Close()
	configURL, err := windows.UTF16PtrFromString(pac.URL + "/proxy.pac")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ target, want string }{
		{"https://api.steampowered.com/auth", "http://127.0.0.1:10809"},
		{"https://steamcommunity.com/", ""},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.target, nil)
		got, err := automaticProxy(req, userConfig{AutoConfigURL: configURL})
		if err != nil {
			t.Fatal(err)
		}
		actual := ""
		if got != nil {
			actual = got.String()
		}
		if actual != tc.want {
			t.Fatalf("proxy=%q, want %q", actual, tc.want)
		}
	}
}

func TestCanceledRequestDoesNotResolve(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://api.steampowered.com", nil)
	ctx, cancel := context.WithTimeout(req.Context(), time.Second)
	cancel()
	if _, err := Proxy(req.WithContext(ctx)); err != context.Canceled {
		t.Fatalf("error=%v", err)
	}
}

func TestAutomaticProxyFailuresDoNotBypassConfiguredProxy(t *testing.T) {
	manual, _ := windows.UTF16PtrFromString("127.0.0.1:10809")
	pac, _ := windows.UTF16PtrFromString("http://config.example/proxy.pac")
	req := httptest.NewRequest(http.MethodGet, "https://api.steampowered.com", nil)
	failed := errors.New("PAC failed")
	for _, tc := range []struct {
		name            string
		config          userConfig
		resolutionError error
		want            string
		wantError       bool
	}{
		{"manual", userConfig{Proxy: manual}, nil, "http://127.0.0.1:10809", false},
		{"disabled", userConfig{}, nil, "", false},
		{"no WPAD uses manual", userConfig{AutoDetect: 1, Proxy: manual}, windows.Errno(12180), "http://127.0.0.1:10809", false},
		{"no WPAD no manual", userConfig{AutoDetect: 1}, windows.Errno(12180), "", false},
		{"PAC failure refuses direct", userConfig{AutoConfigURL: pac}, failed, "", true},
		{"PAC failure refuses manual fallback", userConfig{AutoConfigURL: pac, Proxy: manual}, failed, "", true},
		{"WPAD script failure refuses direct", userConfig{AutoDetect: 1}, failed, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := proxyForWindowsConfig(req, tc.config, func(*http.Request, userConfig) (*url.URL, error) {
				return nil, tc.resolutionError
			})
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			actual := ""
			if got != nil {
				actual = got.String()
			}
			if actual != tc.want {
				t.Fatalf("proxy=%q, want %q", actual, tc.want)
			}
		})
	}
}
