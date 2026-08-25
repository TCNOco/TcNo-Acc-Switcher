package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

func TestCompositeAssetHandlerSetsSecurityHeaders(t *testing.T) {
	handler := newCompositeAssetHandler(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>test</title>")},
	})
	request := httptest.NewRequest(http.MethodGet, "http://wails.localhost/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	policy := response.Header().Get("Content-Security-Policy")
	for _, directive := range []string{
		"default-src 'none'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"script-src 'self'",
		"connect-src 'self'",
		"img-src 'self' data:",
		"media-src 'self'",
		"worker-src 'none'",
		"form-action 'none'",
		"sandbox allow-scripts allow-same-origin",
	} {
		if !strings.Contains(policy, directive) {
			t.Fatalf("CSP missing %q: %q", directive, policy)
		}
	}
	if got := response.Header().Get("Permissions-Policy"); !strings.Contains(got, "camera=()") || !strings.Contains(got, "clipboard-read=()") {
		t.Fatalf("Permissions-Policy = %q", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if strings.Contains(policy, "https:") || strings.Contains(policy, "blob:") {
		t.Fatalf("CSP permits remote or blob content: %q", policy)
	}
	if got := response.Header().Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Opener-Policy = %q", got)
	}
}

func TestWritableAssetAllowedOnlyForMediaDirectories(t *testing.T) {
	for _, path := range []string{"img/profiles/avatar.webp", "backgrounds/app.png", "img/video.webm"} {
		if !writableAssetAllowed(path) {
			t.Errorf("writableAssetAllowed(%q) = false", path)
		}
	}
	for _, path := range []string{"index.html", "assets/app.js", "img/payload.js", "../img/avatar.png"} {
		if writableAssetAllowed(path) {
			t.Errorf("writableAssetAllowed(%q) = true", path)
		}
	}
}

// benchAssetHandler serves a small embedded set with the paths pointed at a
// temp dir, so the disk-override branch can resolve.
func benchAssetHandler(tb testing.TB) http.Handler {
	tb.Helper()
	exeDir := tb.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
	return newCompositeAssetHandler(fstest.MapFS{
		"index.html":          &fstest.MapFile{Data: []byte("<!doctype html><title>t</title>")},
		"assets/index.js":     &fstest.MapFile{Data: []byte("export default 1;")},
		"img/placeholder.png": &fstest.MapFile{Data: []byte("png")},
	})
}

// BenchmarkAssetRequest covers both shapes the handler sees. Scripts,
// stylesheets and fonts can never be served from disk; avatars can, and an
// account page asks for one per row.
func BenchmarkAssetRequest(b *testing.B) {
	handler := benchAssetHandler(b)

	for _, tc := range []struct{ name, path string }{
		{"Embedded", "/assets/index.js"},
		{"Image", "/img/placeholder.png"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				req := httptest.NewRequest(http.MethodGet, "http://wails.localhost"+tc.path, nil)
				handler.ServeHTTP(httptest.NewRecorder(), req)
			}
		})
	}
}
