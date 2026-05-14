package winutil

import (
	"embed"
	"errors"
	iofs "io/fs"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// ErrNoEmbeddedFrontend is returned when SetEmbeddedFrontendFS was not called.
var ErrNoEmbeddedFrontend = errors.New("embedded frontend assets not configured")

// ErrNoPlatformArt is returned when no matching platform image exists in embed.
var ErrNoPlatformArt = errors.New("no platform art in embedded assets")

var (
	embeddedFrontendMu sync.RWMutex
	embeddedFrontend   embed.FS

	platformArtMu   sync.RWMutex
	platformArtOnce sync.Once
	platformArtSVGs map[string]string // normalized key -> svg path
	platformArtPNGs map[string]string // normalized key -> png path
)

// SetEmbeddedFrontendFS sets the embedded Vite dist FS (e.g. main's //go:embed all:frontend/dist).
func SetEmbeddedFrontendFS(f embed.FS) {
	embeddedFrontendMu.Lock()
	embeddedFrontend = f
	embeddedFrontendSet = true
	embeddedFrontendMu.Unlock()
}

func buildPlatformArtMap(rootFS embed.FS) {
	platformArtOnce.Do(func() {
		svgMap := make(map[string]string)
		pngMap := make(map[string]string)
		iofs.WalkDir(rootFS, ".", func(path string, d iofs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !embeddedPathIsUnderPlatformArt(path) {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			norm := normalizePlatformArtKey(stem)
			if norm == "" {
				return nil
			}
			switch ext {
			case ".svg":
				svgMap[norm] = path
			case ".png":
				pngMap[norm] = path
			}
			return nil
		})
		platformArtSVGs = svgMap
		platformArtPNGs = pngMap
	})
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

// embeddedPathIsUnderPlatformArt reports whether path belongs to the platform
// tile directory inside the embedded Vite dist. main embeds //go:embed
// all:frontend/dist, so paths are like "frontend/dist/img/platform/Steam.svg".
// Some layouts use "img/platform/..." at the FS root; accept both.
func embeddedPathIsUnderPlatformArt(path string) bool {
	up := strings.ReplaceAll(path, `\`, "/")
	low := strings.ToLower(up)
	return strings.HasPrefix(low, "frontend/dist/img/platform/") ||
		strings.HasPrefix(low, "img/platform/")
}

// normalizePlatformArtKey folds case and strips non-alphanumeric characters so
// e.g. "Battle.net" (Platforms.json) matches "BattleNet.svg" on disk.
func normalizePlatformArtKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// PlatformArtStemMatchesKey reports whether a filename stem under img/platform
// corresponds to the given platform key from Platforms.json.
func PlatformArtStemMatchesKey(stem, platformKey string) bool {
	key := strings.TrimSpace(platformKey)
	stem = strings.TrimSpace(stem)
	if key == "" || stem == "" {
		return false
	}
	if strings.EqualFold(stem, key) {
		return true
	}
	return normalizePlatformArtKey(stem) == normalizePlatformArtKey(key)
}

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
	buildPlatformArtMap(rootFS)

	norm := normalizePlatformArtKey(key)
	platformArtMu.RLock()
	svgPath := platformArtSVGs[norm]
	pngPath := platformArtPNGs[norm]
	platformArtMu.RUnlock()

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
