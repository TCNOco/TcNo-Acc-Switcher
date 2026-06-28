package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestConfigureWindowsWebViewCachePrunesVolatileCachesOnly(t *testing.T) {
	root := t.TempDir()
	volatile := []string{
		filepath.Join(root, "EBWebView", "Default", "Code Cache", "js", "cached-bytecode"),
		filepath.Join(root, "EBWebView", "Default", "Cache", "Cache_Data", "data_0"),
		filepath.Join(root, "EBWebView", "Default", "GPUCache", "shader.bin"),
		filepath.Join(root, "EBWebView", "Default", "Service Worker", "CacheStorage", "entry"),
		filepath.Join(root, "EBWebView", "ShaderCache", "shader.bin"),
	}
	preserved := []string{
		filepath.Join(root, "EBWebView", "Default", "Local Storage", "leveldb", "000003.log"),
		filepath.Join(root, "EBWebView", "Default", "IndexedDB", "wails.leveldb", "000003.log"),
		filepath.Join(root, "EBWebView", "Default", "Cookies"),
	}

	for _, path := range append(volatile, preserved...) {
		writeTestFile(t, path)
	}

	var opts application.Options
	configureWindowsWebViewCache(&opts, root)

	if opts.Windows.WebviewUserDataPath != root {
		t.Fatalf("WebviewUserDataPath = %q, want %q", opts.Windows.WebviewUserDataPath, root)
	}
	for _, arg := range []string{"--disk-cache-size=1048576", "--media-cache-size=1048576", "--disable-gpu-shader-disk-cache"} {
		if !slices.Contains(opts.Windows.AdditionalBrowserArgs, arg) {
			t.Fatalf("AdditionalBrowserArgs missing %q in %#v", arg, opts.Windows.AdditionalBrowserArgs)
		}
	}
	for _, path := range volatile {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("volatile cache path still exists or stat failed unexpectedly: %s err=%v", path, err)
		}
	}
	for _, path := range preserved {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preserved path missing: %s err=%v", path, err)
		}
	}
}

func TestWebViewCacheChildPathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	if _, ok := webviewCacheChildPath(root, filepath.Join("..", "outside")); ok {
		t.Fatal("expected parent escape to be rejected")
	}
	if _, ok := webviewCacheChildPath(root, filepath.Join(root, "absolute")); ok {
		t.Fatal("expected absolute child path to be rejected")
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
