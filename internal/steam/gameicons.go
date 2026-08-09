package steam

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"

	"golang.org/x/sync/semaphore"
)

const (
	gameIconURLDir  = "/img/gameicons/"
	gameIconCDNBase = "https://cdn.cloudflare.steamstatic.com/steam/apps/"

	// Header art runs 30-250 KB; the ceiling only exists so a broken or hostile
	// response cannot be streamed into the cache directory unbounded.
	gameIconMaxBytes = 4 << 20

	gameIconWarmConcurrency = 5

	// Plenty of app ids (tools, demos, delisted apps) have no header art at all.
	// Remembering that for a while stops a games-view refresh replaying hundreds
	// of 404s every time it renders.
	gameIconMissTTL = 6 * time.Hour

	// Resolving the Steam root re-reads settings from disk, and a warm pass calls
	// through here once per app id.
	gameIconRootTTL = time.Minute
)

// Steam app ids are uint32, so ten digits is the ceiling. Leading zeros are
// rejected as well: "0730" and "730" would otherwise cache the same game twice.
var reGameAppID = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)

// normalizeAppID accepts only a bare decimal app id. Everything downstream
// concatenates the result into a filesystem path and a URL, so anything that
// could escape either ("..", separators, percent escapes) has to die here.
func normalizeAppID(appID string) (string, bool) {
	id := strings.TrimSpace(appID)
	if !reGameAppID.MatchString(id) {
		return "", false
	}
	return id, true
}

// GameIconURL returns the URL a cached icon for appID is served at, or "" when
// appID is not a plain app id. The asset handler serves any /img/** path with an
// image extension straight out of wwwroot, so nothing has to be routed for this.
func GameIconURL(appID string) string {
	id, ok := normalizeAppID(appID)
	if !ok {
		return ""
	}
	return gameIconURLDir + id + ".jpg"
}

func defaultGameIconCacheDir() (string, error) {
	www, err := paths.WwwrootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(www, "img", "gameicons"), nil
}

// gameIconCacheDir is a var because paths.WwwrootDir memoises process-wide and
// paths.ResetForTest does not clear it; tests point this at a temp dir.
var gameIconCacheDir = defaultGameIconCacheDir

type gameIconDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// gameIconClient is appclient.Shared - offline-aware and pooled. Deliberately not
// the steamguard protocol client: that is a hardened auth transport with a strict
// host allowlist, and bulk artwork has no business on it.
var gameIconClient gameIconDoer = appclient.Shared

var (
	gameIconLibraryMu   sync.Mutex
	gameIconLibraryDir  string
	gameIconLibraryTime time.Time
)

// steamLibraryCacheDir returns <steamRoot>/appcache/librarycache, or "" when the
// Steam root cannot be resolved (Steam not installed, settings unreadable).
func steamLibraryCacheDir() string {
	gameIconLibraryMu.Lock()
	defer gameIconLibraryMu.Unlock()
	if !gameIconLibraryTime.IsZero() && time.Since(gameIconLibraryTime) < gameIconRootTTL {
		return gameIconLibraryDir
	}
	dir := ""
	if root, err := installRoot(); err == nil {
		if root = strings.TrimSpace(root); root != "" {
			dir = filepath.Join(root, "appcache", "librarycache")
		}
	}
	gameIconLibraryDir = dir
	gameIconLibraryTime = time.Now()
	return dir
}

// Header-shaped art only. The portrait capsule and hero images Steam also caches
// have a completely different aspect ratio, and mixing them into one grid with
// the CDN fallback (which is header.jpg) looks broken.
var gameIconLibraryNames = []string{"header.jpg", "library_header.jpg"}

// findLibraryCacheIcon picks the best artwork Steam has already cached for appID.
// Current clients store <librarycache>/<appid>/header.jpg, with localised copies
// under a per-language sha1-named subfolder; builds before the library rework
// wrote flat <appid>_header.jpg / <appid>_icon.jpg files instead. Missing folders
// are normal, not an error.
func findLibraryCacheIcon(libraryCacheDir, appID string) string {
	id, ok := normalizeAppID(appID)
	if !ok || strings.TrimSpace(libraryCacheDir) == "" {
		return ""
	}

	appDir := filepath.Join(libraryCacheDir, id)
	for _, name := range gameIconLibraryNames {
		if p := filepath.Join(appDir, name); isGameIconFile(p) {
			return p
		}
	}

	// Localised variants live one sha1-named folder deep. os.ReadDir sorts by
	// name, so a game with several languages cached always resolves to the same
	// file instead of flickering between them.
	if subs, err := os.ReadDir(appDir); err == nil {
		for _, name := range gameIconLibraryNames {
			for _, e := range subs {
				if !e.IsDir() {
					continue
				}
				if p := filepath.Join(appDir, e.Name(), name); isGameIconFile(p) {
					return p
				}
			}
		}
	}

	for _, suffix := range []string{"_header.jpg", "_icon.jpg"} {
		if p := filepath.Join(libraryCacheDir, id+suffix); isGameIconFile(p) {
			return p
		}
	}
	return ""
}

func isGameIconFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Size() > 0
}

func readGameIconFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, gameIconMaxBytes))
}

// looksLikeJPEG keeps an HTML error page or a truncated body from being cached
// under a .jpg name, which the webview would then render as a broken image.
func looksLikeJPEG(b []byte) bool {
	return len(b) > 4 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
}

var (
	gameIconMissMu sync.Mutex
	gameIconMisses = map[string]time.Time{}
)

func gameIconRecentlyMissing(id string) bool {
	gameIconMissMu.Lock()
	defer gameIconMissMu.Unlock()
	at, ok := gameIconMisses[id]
	if !ok {
		return false
	}
	if time.Since(at) >= gameIconMissTTL {
		delete(gameIconMisses, id)
		return false
	}
	return true
}

func markGameIconMissing(id string) {
	gameIconMissMu.Lock()
	gameIconMisses[id] = time.Now()
	gameIconMissMu.Unlock()
}

// EnsureGameIcon resolves a Steam app id to an icon cached under
// wwwroot/img/gameicons and returns the URL the frontend renders it from.
// Steam's own librarycache is preferred over the CDN. An app id with no artwork
// anywhere is not an error: the URL comes back empty and the caller draws its
// placeholder.
func EnsureGameIcon(ctx context.Context, appID string) (string, error) {
	id, ok := normalizeAppID(appID)
	if !ok {
		return "", fmt.Errorf("steam game icon: invalid app id %q", appID)
	}
	dir, err := gameIconCacheDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(dir, id+".jpg")
	if isGameIconFile(dest) {
		return GameIconURL(id), nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	if src := findLibraryCacheIcon(steamLibraryCacheDir(), id); src != "" {
		data, readErr := readGameIconFile(src)
		switch {
		case readErr != nil:
			steamLog.Debug("steam game icon librarycache read failed",
				slog.String("appId", id), slog.String("src", src), slog.Any("err", readErr))
		case !looksLikeJPEG(data):
			steamLog.Debug("steam game icon librarycache entry is not a jpeg",
				slog.String("appId", id), slog.String("src", src))
		default:
			if err := fsutil.WriteFileAtomic(dest, data, 0o644); err != nil {
				return "", err
			}
			return GameIconURL(id), nil
		}
	}

	// Below the librarycache copy on purpose: offline mode blocks the network,
	// not the disk, and art Steam has already downloaded still resolves. Tested
	// per app id rather than once per warm pass, so a pass of a few thousand
	// icons stops downloading the moment offline is switched on. Nothing is
	// marked missing here - being offline says nothing about what the CDN holds.
	if appclient.IsOfflineMode() {
		steamLog.Debug("steam game icon download skipped: offline mode", slog.String("appId", id))
		return "", nil
	}
	if gameIconRecentlyMissing(id) {
		return "", nil
	}

	data, err := downloadGameIcon(ctx, id)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		markGameIconMissing(id)
		return "", nil
	}
	if err := fsutil.WriteFileAtomic(dest, data, 0o644); err != nil {
		return "", err
	}
	return GameIconURL(id), nil
}

// downloadGameIcon returns nil, nil when the CDN has no header art for this app.
// That is the ordinary outcome for tools, demos and delisted apps, so it stays at
// debug level rather than being reported as a failure.
func downloadGameIcon(ctx context.Context, id string) ([]byte, error) {
	url := gameIconCDNBase + id + "/header.jpg"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/jpeg,image/*")
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher/3 (Steam game art; +https://github.com/TcNo-Acc-Switcher)")

	resp, err := gameIconClient.Do(req)
	if err != nil {
		// Offline mode included, deliberately: reporting it as nil, nil would file
		// the app id under "no artwork anywhere" for the whole miss TTL, so the
		// icon would still be absent hours after the user came back online.
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		steamLog.Debug("steam game icon not on cdn", slog.String("appId", id), slog.Int("status", resp.StatusCode))
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, gameIconMaxBytes))
	if err != nil {
		return nil, err
	}
	if !looksLikeJPEG(data) {
		steamLog.Debug("steam game icon cdn response is not a jpeg",
			slog.String("appId", id), slog.Int("bytes", len(data)))
		return nil, nil
	}
	return data, nil
}

// WarmGameIcons caches icons for appIDs with bounded concurrency and returns the
// ones that resolved; ids with no artwork are simply absent from the map.
// It can block on the network for as long as ctx allows, so it must not be called
// from a UI path - hand it a background context and let the view repaint later.
func WarmGameIcons(ctx context.Context, appIDs []string) map[string]string {
	out := make(map[string]string, len(appIDs))
	if len(appIDs) == 0 {
		return out
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(gameIconWarmConcurrency)
	seen := make(map[string]struct{}, len(appIDs))

	for _, raw := range appIDs {
		id, ok := normalizeAppID(raw)
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}

		// Acquire before spawning so a cancelled ctx stops the batch here instead
		// of queueing the whole remaining list as goroutines.
		if err := sem.Acquire(ctx, 1); err != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer crashlog.Capture()
			defer wg.Done()
			defer sem.Release(1)

			url, err := EnsureGameIcon(ctx, id)
			if err != nil {
				steamLog.Debug("steam game icon warm failed", slog.String("appId", id), slog.Any("err", err))
				return
			}
			if url == "" {
				return
			}
			mu.Lock()
			out[id] = url
			mu.Unlock()
		}()
	}

	wg.Wait()
	return out
}
