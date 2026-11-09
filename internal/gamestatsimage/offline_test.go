package gamestatsimage

import (
	"context"
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

func TestExpiredGameStatImageSurvivesOfflineRefresh(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	dir, err := cacheDir("gs/test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rank.png")
	if err := os.WriteFile(path, []byte("cached-rank"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: offlineTransport(func(*http.Request) (*http.Response, error) { return nil, &net.DNSError{Err: "no such host"} })}
	url, err := DownloadIfNeeded(context.Background(), client, "gs/test", "https://example.invalid/rank.png", 7)
	if err != nil || url != PublicURL("gs/test", "rank.png") {
		t.Fatalf("cached URL lost: %q %v", url, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "cached-rank" {
		t.Fatal("offline refresh changed cached image")
	}
}
