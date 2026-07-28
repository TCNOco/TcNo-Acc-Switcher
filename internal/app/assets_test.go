package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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
