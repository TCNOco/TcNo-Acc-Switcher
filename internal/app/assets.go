package app

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const contentSecurityPolicy = "default-src 'none'; base-uri 'none'; object-src 'none'; frame-src 'none'; frame-ancestors 'none'; form-action 'none'; script-src 'self'; script-src-attr 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; media-src 'self'; font-src 'self'; connect-src 'self'; worker-src 'none'; manifest-src 'none'; sandbox allow-scripts allow-same-origin"

var writableAssetExtensions = map[string]struct{}{
	".avif": {}, ".bmp": {}, ".gif": {}, ".ico": {}, ".jpeg": {}, ".jpg": {},
	".mp4": {}, ".png": {}, ".svg": {}, ".webm": {}, ".webp": {},
}

func writableAssetAllowed(path string) bool {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	if !strings.HasPrefix(clean, "img/") && !strings.HasPrefix(clean, "backgrounds/") {
		return false
	}
	_, ok := writableAssetExtensions[strings.ToLower(filepath.Ext(clean))]
	return ok
}

var wwwrootOnce sync.Once

func ensureWwwroot(dir string) {
	wwwrootOnce.Do(func() { _ = os.MkdirAll(dir, 0o755) })
}

func newCompositeAssetHandler(embedded fs.FS) http.Handler {
	embedHandler := application.AssetFileServerFS(embedded)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), notifications=(), clipboard-read=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Origin-Agent-Cluster", "?1")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			embedHandler.ServeHTTP(w, r)
			return
		}
		// Classify before touching the filesystem. Only img/ and backgrounds/
		// can be served from disk, so every script, stylesheet and font - and
		// there are far more of those than there are avatars - was resolving the
		// wwwroot path and running MkdirAll for a branch it could never take.
		upath := strings.TrimPrefix(filepath.ToSlash(r.URL.Path), "/")
		if upath == "" {
			upath = "."
		}
		if !writableAssetAllowed(upath) {
			embedHandler.ServeHTTP(w, r)
			return
		}

		wwwroot, err := platform.WwwrootDir()
		if err != nil {
			embedHandler.ServeHTTP(w, r)
			return
		}
		// Once per process: the directory does not come and go, and everything
		// that writes into it creates what it needs itself. An account page asks
		// for one avatar per row, and each of those was a syscall.
		ensureWwwroot(wwwroot)
		diskPath := filepath.Join(wwwroot, filepath.FromSlash(upath))
		wwwClean := filepath.Clean(wwwroot)
		diskClean := filepath.Clean(diskPath)
		rel, err := filepath.Rel(wwwClean, diskClean)
		if err != nil || strings.HasPrefix(rel, "..") {
			embedHandler.ServeHTTP(w, r)
			return
		}
		st, err := os.Stat(diskClean)
		if err == nil && !st.IsDir() {
			http.ServeFile(w, r, diskClean)
			return
		}
		embedHandler.ServeHTTP(w, r)
	})
}
