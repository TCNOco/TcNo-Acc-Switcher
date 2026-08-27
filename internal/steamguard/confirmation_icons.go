package steamguard

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationicon"
)

// The webview's CSP is `img-src 'self' data:`, so a Steam URL cannot be rendered
// directly. Icons are fetched here, sanitized, and written under wwwroot to be
// served from the app's own origin.
const confirmationIconDirName = "confirmations"

// steamIconHosts is the exact-host allowlist for confirmation icons. Steam serves
// economy and avatar images from its own CDN fronts; anything else is refused
// before a connection is made.
var steamIconHosts = []string{
	"community.cloudflare.steamstatic.com",
	"community.akamai.steamstatic.com",
	"community.fastly.steamstatic.com",
	"community.steamstatic.com",
	"avatars.cloudflare.steamstatic.com",
	"avatars.akamai.steamstatic.com",
	"avatars.fastly.steamstatic.com",
	"avatars.steamstatic.com",
	"cdn.cloudflare.steamstatic.com",
	"cdn.akamai.steamstatic.com",
	"cdn.fastly.steamstatic.com",
	"steamcommunity-a.akamaihd.net",
	"steamcdn-a.akamaihd.net",
}

func steamIconPolicy() confirmationicon.Policy {
	return confirmationicon.Policy{
		AllowedHosts:   steamIconHosts,
		MaxInputBytes:  2 << 20,
		MaxOutputBytes: 2 << 20,
		MaxWidth:       512,
		MaxHeight:      512,
		MaxPixels:      512 * 512,
		Timeout:        15 * time.Second,
	}
}

// confirmationIconCache turns remote icon URLs into files the webview may load.
//
// It holds only what the open list refers to: every refresh prunes files no
// longer referenced, and closing the window clears the folder entirely.
type confirmationIconCache struct {
	mu      sync.Mutex
	fetcher *confirmationicon.Fetcher
	dir     string
}

func newConfirmationIconCache() *confirmationIconCache {
	return &confirmationIconCache{}
}

// ensureReady prepares the fetcher and directory on first use, so a run that
// never opens confirmations pays nothing.
func (c *confirmationIconCache) ensureReady() error {
	if c.fetcher != nil && c.dir != "" {
		return nil
	}
	wwwroot, err := paths.WwwrootDir()
	if err != nil {
		return err
	}
	directory := filepath.Join(wwwroot, "img", confirmationIconDirName)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	fetcher, err := confirmationicon.New(steamIconPolicy())
	if err != nil {
		return err
	}
	c.fetcher = fetcher
	c.dir = directory
	return nil
}

// localName is the on-disk name for a remote URL: a digest, so the file reveals
// nothing about what was traded and the same icon is fetched once.
func localName(rawURL, extension string) string {
	digest := sha256.Sum256([]byte(rawURL))
	return base64.RawURLEncoding.EncodeToString(digest[:16]) + extension
}

func extensionForMIME(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ""
	}
}

// Ensure returns the app-relative URL for one icon, fetching it if it is not
// already cached. It returns "" whenever the icon cannot be served, which the UI
// renders as its own placeholder rather than as an error.
func (c *confirmationIconCache) Ensure(ctx context.Context, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureReady(); err != nil {
		confirmationsLogger().Warn("confirmation icon cache unavailable", "error", err)
		return ""
	}
	for _, extension := range []string{".png", ".jpg", ".webp", ".gif"} {
		name := localName(rawURL, extension)
		if _, err := os.Stat(filepath.Join(c.dir, name)); err == nil {
			return "/img/" + confirmationIconDirName + "/" + name
		}
	}

	result, err := c.fetcher.Fetch(ctx, rawURL)
	if err != nil || result.State != confirmationicon.StateReady {
		// Kind only: the URL itself describes what is being traded.
		confirmationsLogger().Debug("confirmation icon not served", "reason", string(result.Reason))
		return ""
	}
	extension := extensionForMIME(result.MIME)
	if extension == "" {
		return ""
	}
	name := localName(rawURL, extension)
	if err := os.WriteFile(filepath.Join(c.dir, name), result.Data, 0o600); err != nil {
		confirmationsLogger().Warn("confirmation icon could not be cached", "error", err)
		return ""
	}
	return "/img/" + confirmationIconDirName + "/" + name
}

// Prune deletes every cached icon that the given local URLs do not refer to. It
// is called after each refresh, so an icon outlives its confirmation by one list
// at most.
func (c *confirmationIconCache) Prune(keep []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dir == "" {
		return
	}
	wanted := make(map[string]struct{}, len(keep))
	for _, url := range keep {
		if name := path.Base(url); name != "" && name != "." && name != "/" {
			wanted[name] = struct{}{}
		}
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := wanted[entry.Name()]; ok {
			continue
		}
		_ = os.Remove(filepath.Join(c.dir, entry.Name()))
	}
}

// Clear empties the cache: the confirmations window closing is the point at
// which none of these images are needed any more.
func (c *confirmationIconCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dir == "" {
		return
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			_ = os.Remove(filepath.Join(c.dir, entry.Name()))
		}
	}
}
