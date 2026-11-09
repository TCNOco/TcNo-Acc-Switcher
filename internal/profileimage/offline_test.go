package profileimage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

type offlineTransport func(*http.Request) (*http.Response, error)

func (f offlineTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestExpiredAvatarSurvivesFailedRefreshAndRecovers(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	dir, err := ProfileDir("Steam")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "account.jpg")
	old := []byte("last-good-avatar")
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(path, expired, expired); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		err    error
		status int
		body   []byte
	}{
		{name: "DNS failure", err: &net.DNSError{Err: "no such host"}},
		{name: "timeout", err: context.DeadlineExceeded},
		{name: "server failure", status: 503},
		{name: "empty response", status: 200},
		{name: "captive portal", status: 200, body: []byte("<html>Wi-Fi sign in</html>")},
		{name: "oversized response", status: 200, body: bytes.Repeat([]byte{1}, (20<<20)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: offlineTransport(func(*http.Request) (*http.Response, error) {
				if tc.err != nil {
					return nil, tc.err
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewReader(tc.body))}, nil
			})}
			result, err := DownloadIfNeeded(context.Background(), client, "Steam", "account", "https://example.invalid/avatar.jpg", 7)
			if err != nil || result == nil || result.LocalPath != path || !result.Stale {
				t.Fatalf("cached avatar not returned: result=%v err=%v", result, err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, old) {
				t.Fatalf("cached bytes changed: err=%v", err)
			}
			if !FileOlderThanDays(path, 7) {
				t.Fatal("failed refresh made stale data fresh")
			}
		})
	}
	client := &http.Client{Transport: offlineTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewBufferString("new-avatar"))}, nil
	})}
	if _, err := DownloadIfNeeded(context.Background(), client, "Steam", "account", "https://example.invalid/avatar.jpg", 7); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new-avatar" {
		t.Fatal("reconnection did not replace the stale image")
	}
	client.Transport = offlineTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("unreachable") })
	if _, err := DownloadIfNeeded(context.Background(), client, "Steam", "missing", "https://example.invalid/missing.jpg", 7); err == nil {
		t.Fatal("a failed first download was reported as cached success")
	}
}
