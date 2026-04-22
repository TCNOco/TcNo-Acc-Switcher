package winutil

import (
	"embed"
	"errors"
	iofs "io/fs"
	"path/filepath"
	"strings"
	"sync"
)

// ErrNoEmbeddedFrontend is returned when SetEmbeddedFrontendFS was not called.
var ErrNoEmbeddedFrontend = errors.New("embedded frontend assets not configured")

// ErrNoPlatformArt is returned when no matching platform image exists in embed.
var ErrNoPlatformArt = errors.New("no platform art in embedded assets")

var (
	embeddedFrontendMu sync.RWMutex
	embeddedFrontend   embed.FS
)

// SetEmbeddedFrontendFS sets the embedded Vite dist FS (e.g. main's //go:embed all:frontend/dist).
func SetEmbeddedFrontendFS(f embed.FS) {
	embeddedFrontendMu.Lock()
	embeddedFrontend = f
	embeddedFrontendSet = true
	embeddedFrontendMu.Unlock()
}

func embeddedFS() (embed.FS, bool) {
	embeddedFrontendMu.RLock()
	defer embeddedFrontendMu.RUnlock()
	// embed.FS zero value is usable but empty; we track explicit SetEmbeddedFrontendFS via a flag.
	if !embeddedFrontendSet {
		return embed.FS{}, false
	}
	return embeddedFrontend, true
}

var embeddedFrontendSet bool

// FindPlatformArt resolves img/platform/<name>.{svg,png} from embedded dist (case-insensitive stem match).
func FindPlatformArt(platformKey string) (svg []byte, png []byte, err error) {
	rootFS, ok := embeddedFS()
	if !ok {
		return nil, nil, ErrNoEmbeddedFrontend
	}
	key := strings.TrimSpace(platformKey)
	if key == "" {
		return nil, nil, ErrNoPlatformArt
	}
	var svgPath, pngPath string
	_ = iofs.WalkDir(rootFS, ".", func(path string, d iofs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		low := strings.ToLower(path)
		if !strings.HasPrefix(low, "img/platform/") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if !strings.EqualFold(stem, key) {
			return nil
		}
		switch ext {
		case ".svg":
			svgPath = path
		case ".png":
			pngPath = path
		}
		return nil
	})
	if svgPath != "" {
		svg, err = rootFS.ReadFile(svgPath)
		if err != nil {
			return nil, nil, err
		}
	}
	if pngPath != "" {
		png, err = rootFS.ReadFile(pngPath)
		if err != nil {
			return nil, nil, err
		}
	}
	if len(svg) == 0 && len(png) == 0 {
		return nil, nil, ErrNoPlatformArt
	}
	return svg, png, nil
}
