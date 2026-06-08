package paths

import (
	"path/filepath"
	"testing"
)

func TestWebViewCacheDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "TcNo Account Switcher")
	ResetForTest(dataDir)

	got, err := WebViewCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dataDir, "WebViewCache")
	if got != want {
		t.Fatalf("WebViewCacheDir() = %q, want %q", got, want)
	}
}
