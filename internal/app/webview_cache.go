package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const webviewCacheSizeBytes = 1 * 1024 * 1024

var webviewRuntimeCacheDirs = []string{
	filepath.Join("EBWebView", "Default", "Code Cache"),
	filepath.Join("EBWebView", "Default", "Cache"),
	filepath.Join("EBWebView", "Default", "GPUCache"),
	filepath.Join("EBWebView", "Default", "DawnCache"),
	filepath.Join("EBWebView", "Default", "blob_storage"),
	filepath.Join("EBWebView", "Default", "Service Worker", "CacheStorage"),
	filepath.Join("EBWebView", "Default", "Service Worker", "ScriptCache"),
	filepath.Join("EBWebView", "GrShaderCache"),
	filepath.Join("EBWebView", "ShaderCache"),
}

func configureWindowsWebViewCache(appOpts *application.Options, cacheDir string) {
	pruneWebViewRuntimeCaches(cacheDir)
	appOpts.Windows.WebviewUserDataPath = cacheDir
	appOpts.Windows.AdditionalBrowserArgs = append(appOpts.Windows.AdditionalBrowserArgs,
		fmt.Sprintf("--disk-cache-size=%d", webviewCacheSizeBytes),
		fmt.Sprintf("--media-cache-size=%d", webviewCacheSizeBytes),
		"--disable-gpu-shader-disk-cache",
	)
}

func pruneWebViewRuntimeCaches(root string) {
	for _, rel := range webviewRuntimeCacheDirs {
		target, ok := webviewCacheChildPath(root, rel)
		if !ok {
			log.Printf("webview cache prune skipped unsafe path: root=%q rel=%q", root, rel)
			continue
		}
		if err := os.RemoveAll(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("webview cache prune %s: %v", target, err)
		}
	}
}

func webviewCacheChildPath(root, rel string) (string, bool) {
	if strings.TrimSpace(root) == "" || filepath.IsAbs(rel) {
		return "", false
	}
	base, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", false
	}
	target := filepath.Join(base, rel)
	inside, err := filepath.Rel(base, target)
	if err != nil || inside == "." || inside == ".." || filepath.IsAbs(inside) || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return target, true
}
