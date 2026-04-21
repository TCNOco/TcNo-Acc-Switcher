package main

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func newCompositeAssetHandler(embedded fs.FS) http.Handler {
	embedHandler := application.AssetFileServerFS(embedded)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			embedHandler.ServeHTTP(w, r)
			return
		}
		wwwroot, err := platform.WwwrootDir()
		if err != nil {
			embedHandler.ServeHTTP(w, r)
			return
		}
		_ = os.MkdirAll(wwwroot, 0o755)

		upath := strings.TrimPrefix(filepath.ToSlash(r.URL.Path), "/")
		if upath == "" {
			upath = "."
		}
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
