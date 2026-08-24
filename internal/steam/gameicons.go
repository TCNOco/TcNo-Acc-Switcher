package steam

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
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
	"TcNo-Acc-Switcher/internal/paths"

	"golang.org/x/sync/semaphore"
)

const (
	// One level below img/gameicons on purpose. Earlier builds cached 460x215
	// header art as img/gameicons/<appid>.jpg; those files are still on disk and
	// must never be served into a slot that now promises a square icon, so the
	// new scheme takes its own directory and the old one is pruned.
	gameIconURLDir  = "/img/gameicons/square/"
	gameIconCDNBase = "https://cdn.cloudflare.steamstatic.com/steam/apps/"

	// Client icons are about 1 KB and header art 30-250 KB; the ceiling only
	// exists so a broken or hostile response cannot be streamed into the cache
	// directory unbounded.
	gameIconMaxBytes = 4 << 20

	// image.DecodeConfig only needs the JPEG headers, which sit in the first few
	// KiB. The cap stops a corrupt file from being walked to its end once per
	// candidate, per app, across a whole library.
	gameIconDimensionScanBytes = 128 << 10

	gameIconWarmConcurrency = 5

	// The local pass is disk-bound, not network-bound, so it runs wider than the
	// download fan-out. Past about this width it stops paying: the cost is NTFS
	// file creation and the AV filter driver, which do not parallelise.
	gameIconLocalConcurrency = 16

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
	return filepath.Join(www, "img", "gameicons", "square"), nil
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

// steamLibraryCacheDir returns <steamRoot>/appcache/librarycache, or "" when no
// configured Steam root has one (Steam not installed, settings unreadable).
//
// The root is never hardcoded here: it comes from the same settings and
// Platforms.json chain the rest of the package resolves against, so a portable
// install or a Steam on another drive is found. Every candidate in that chain is
// tried rather than only the first, because the winning one is
// SteamSettings.FolderPath, which holds a shipped Windows default until the user
// changes it.
func steamLibraryCacheDir() string {
	gameIconLibraryMu.Lock()
	defer gameIconLibraryMu.Unlock()
	if !gameIconLibraryTime.IsZero() && time.Since(gameIconLibraryTime) < gameIconRootTTL {
		return gameIconLibraryDir
	}

	dir := ""
	for _, root := range steamRootCandidatesFn() {
		candidate := filepath.Join(root, "appcache", "librarycache")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			dir = candidate
			break
		}
	}
	// Nothing verified. Fall back to whatever the ordinary resolution settles on
	// so a Steam that keeps its artwork somewhere unexpected still gets a look in.
	if dir == "" {
		if root, err := installRoot(); err == nil {
			if root = strings.TrimSpace(root); root != "" {
				dir = filepath.Join(root, "appcache", "librarycache")
			}
		}
	}

	gameIconLibraryDir = dir
	gameIconLibraryTime = time.Now()
	return dir
}

// Header-shaped art, used only when no square icon exists. The portrait capsule
// and hero images Steam also caches are further from square still, so they are
// not candidates at all.
var gameIconHeaderNames = []string{"header.jpg", "library_header.jpg"}

// Steam names the cached client icon after the app's img_icon_url hash: 40 hex
// characters. The file is byte-identical to what the community CDN serves at
// steamcommunity/public/images/apps/<appid>/<img_icon_url>.jpg.
//
// The hash name is what distinguishes the icon from the rest of an app folder,
// which is otherwise a fixed vocabulary of descriptive names - header.jpg,
// library_600x900.jpg, library_hero.jpg, library_hero_blur.jpg,
// library_header.jpg, logo.png. Being 40 characters it is also the longest name
// present, which is the rule by eye, but the hash shape is the real one.
var reLibraryIconName = regexp.MustCompile(`^[0-9a-f]{40}\.jpg$`)

// findLibraryCacheIcon picks the best artwork Steam has already cached for appID,
// preferring a square icon over header-shaped art.
//
// Current clients store <librarycache>/<appid>/<img_icon_url>.jpg (the 32x32
// client icon) alongside header.jpg and the hero and portrait art, with
// localised header copies under a per-language sha1-named subfolder. Builds
// before the library rework wrote flat <appid>_icon.jpg / <appid>_header.jpg
// files instead. Missing folders are normal, not an error.
func findLibraryCacheIcon(libraryCacheDir, appID string) string {
	if p := findLibraryCacheSquareIcon(libraryCacheDir, appID); p != "" {
		return p
	}
	return findLibraryCacheHeader(libraryCacheDir, appID)
}

// findLibraryCacheSquareIcon returns the square client icon Steam has cached for
// appID, or "" when there is none. Split from the header fallback because it is
// the cheap half: a 32x32 icon is about a kilobyte, where header art runs to
// hundreds, and only this half is fast enough to run before a paint.
func findLibraryCacheSquareIcon(libraryCacheDir, appID string) string {
	id, ok := normalizeAppID(appID)
	if !ok || strings.TrimSpace(libraryCacheDir) == "" {
		return ""
	}
	appDir := filepath.Join(libraryCacheDir, id)
	// os.ReadDir sorts by name, so an app with several cached variants always
	// resolves to the same file instead of flickering between them.
	entries, _ := os.ReadDir(appDir)
	if p := findSquareClientIcon(appDir, entries); p != "" {
		return p
	}
	// The legacy flat layout kept the square icon under its own name.
	if p := filepath.Join(libraryCacheDir, id+"_icon.jpg"); isSquareGameIconFile(p) {
		return p
	}
	return ""
}

// findLibraryCacheHeader returns the header-shaped art Steam has cached for
// appID, which the games view crops into its square slot when the app has no
// square icon at all.
func findLibraryCacheHeader(libraryCacheDir, appID string) string {
	id, ok := normalizeAppID(appID)
	if !ok || strings.TrimSpace(libraryCacheDir) == "" {
		return ""
	}
	appDir := filepath.Join(libraryCacheDir, id)
	entries, _ := os.ReadDir(appDir)

	for _, name := range gameIconHeaderNames {
		if p := filepath.Join(appDir, name); isGameIconFile(p) {
			return p
		}
	}
	for _, name := range gameIconHeaderNames {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if p := filepath.Join(appDir, e.Name(), name); isGameIconFile(p) {
				return p
			}
		}
	}
	if p := filepath.Join(libraryCacheDir, id+"_header.jpg"); isGameIconFile(p) {
		return p
	}
	return ""
}

// findSquareClientIcon returns the smallest square icon cached in appDir, or ""
// when there is none.
//
// 25 of 5947 app folders measured here hold a second square asset beside the
// 32x32 client icon, running from 500px to 3840px. The small one is the icon
// meant for a list row; the large one is a 2-3 MB poster that would be decoded
// down to a 32px slot and cost tens of MB of wwwroot to keep. The shape is
// measured rather than inferred from the name or the file size, because only the
// shape is what this has to promise.
func findSquareClientIcon(appDir string, entries []os.DirEntry) string {
	best, bestWidth := "", 0
	for _, e := range entries {
		if e.IsDir() || !reLibraryIconName.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(appDir, e.Name())
		width, square := squareImageWidth(path)
		if !square || (best != "" && width >= bestWidth) {
			continue
		}
		best, bestWidth = path, width
	}
	return best
}

// squareImageWidth reports the edge length of path when it is a readable square
// image. Anything unreadable, empty or off-square comes back as not square.
func squareImageWidth(path string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(io.LimitReader(f, gameIconDimensionScanBytes))
	if err != nil || cfg.Width <= 0 || cfg.Width != cfg.Height {
		return 0, false
	}
	return cfg.Width, true
}

func isSquareGameIconFile(path string) bool {
	if !isGameIconFile(path) {
		return false
	}
	_, square := squareImageWidth(path)
	return square
}

func isGameIconFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Size() > 0
}

// pruneLegacyGameIcons deletes the flat img/gameicons/<appid>.jpg cache the
// header-art scheme left behind.
//
// Those files are 460x215 banners that nothing reads any more, and a full
// library of them is a few hundred MB sitting in wwwroot forever. Only names
// that are exactly an app id are touched, and failures are ignored: this is
// housekeeping, not a step the icon path depends on.
func pruneLegacyGameIcons(squareDir string) {
	parent := filepath.Dir(squareDir)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name, ok := strings.CutSuffix(e.Name(), ".jpg")
		if !ok || !reGameAppID.MatchString(name) {
			continue
		}
		_ = os.Remove(filepath.Join(parent, e.Name()))
	}
}

var pruneLegacyGameIconsOnce sync.Once

// writeGameIcon publishes an icon by rename, so a reader never sees a partial
// file, but does not fsync it and does not file an action-log entry.
//
// fsutil.WriteFileAtomic does both, which is right for settings and vault data
// and wrong for a derived art cache: on a 4139-icon library the fsyncs alone
// cost seconds, and the log entries bury real file operations under one row per
// thumbnail. Anything lost to a crash is copied again from Steam's own cache on
// the next pass.
func writeGameIcon(dest string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(dest), ".icon-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
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

// ensureGameIconLocal caches appID's icon from disk alone: an existing cache
// entry, or whatever Steam's own librarycache already holds. It never touches
// the network, so it is safe on a UI path.
//
// squareOnly restricts it to the square client icon and skips the header
// fallback. Header art is 20-250 KB against a kilobyte for an icon, and on a
// 5947-app library copying the headers as well takes the pass from 0.4s to 5.5s
// - too long to hold a paint for art that is only ever a cropped stand-in.
//
// An app id with no local artwork returns "" and no error - that is the ordinary
// outcome for the third of a library Steam has never drawn a library page for.
func ensureGameIconLocal(id string, squareOnly bool) (string, error) {
	dir, err := gameIconCacheDir()
	if err != nil {
		return "", err
	}
	pruneLegacyGameIconsOnce.Do(func() { pruneLegacyGameIcons(dir) })
	dest := filepath.Join(dir, id+".jpg")
	if isGameIconFile(dest) {
		return GameIconURL(id), nil
	}

	lc := steamLibraryCacheDir()
	src := findLibraryCacheSquareIcon(lc, id)
	if src == "" && !squareOnly {
		src = findLibraryCacheHeader(lc, id)
	}
	if src == "" {
		return "", nil
	}
	data, err := readGameIconFile(src)
	switch {
	case err != nil:
		steamLog().Debug("steam game icon librarycache read failed",
			slog.String("appId", id), slog.String("src", src), slog.Any("err", err))
		return "", nil
	case !looksLikeJPEG(data):
		steamLog().Debug("steam game icon librarycache entry is not a jpeg",
			slog.String("appId", id), slog.String("src", src))
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := writeGameIcon(dest, data); err != nil {
		return "", err
	}
	return GameIconURL(id), nil
}

// EnsureLocalGameIcon caches appID's icon from Steam's own librarycache,
// falling back to header art when the app has no square client icon. Disk only,
// so unlike EnsureGameIcon it is safe to call from a path that must not block on
// the network. Returns "" when Steam has no artwork cached for the app.
func EnsureLocalGameIcon(appID string) (string, error) {
	id, ok := normalizeAppID(appID)
	if !ok {
		return "", fmt.Errorf("steam game icon: invalid app id %q", appID)
	}
	return ensureGameIconLocal(id, false)
}

// EnsureGameIcon resolves a Steam app id to an icon cached under
// wwwroot/img/gameicons/square and returns the URL the frontend renders it from.
// Steam's own librarycache is preferred over the CDN, and a square client icon
// over header art. An app id with no artwork anywhere is not an error: the URL
// comes back empty and the caller draws its placeholder.
func EnsureGameIcon(ctx context.Context, appID string) (string, error) {
	id, ok := normalizeAppID(appID)
	if !ok {
		return "", fmt.Errorf("steam game icon: invalid app id %q", appID)
	}
	url, err := ensureGameIconLocal(id, false)
	if err != nil || url != "" {
		return url, err
	}

	// Below the librarycache copy on purpose: offline mode blocks the network,
	// not the disk, and art Steam has already downloaded still resolves. Tested
	// per app id rather than once per warm pass, so a pass of a few thousand
	// icons stops downloading the moment offline is switched on. Nothing is
	// marked missing here - being offline says nothing about what the CDN holds.
	if appclient.IsOfflineMode() {
		steamLog().Debug("steam game icon download skipped: offline mode", slog.String("appId", id))
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
	dir, err := gameIconCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := writeGameIcon(filepath.Join(dir, id+".jpg"), data); err != nil {
		return "", err
	}
	return GameIconURL(id), nil
}

// EnsureLocalGameIcons caches every square icon that resolves from Steam's own
// librarycache and returns the URLs keyed by app id; ids with no cached square
// icon are absent from the map and are left to WarmGameIcons, which will fall
// back to header art and then the CDN.
//
// Disk only and bounded, so a view can call it synchronously before its first
// paint. That is the point of it existing separately from WarmGameIcons: mixing
// the two starves the local copies, which are a sub-millisecond file read each,
// behind CDN requests for the app ids that have no local art at all.
func EnsureLocalGameIcons(appIDs []string) map[string]string {
	out := make(map[string]string, len(appIDs))
	if len(appIDs) == 0 {
		return out
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, gameIconLocalConcurrency)
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

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer crashlog.Capture()
			defer wg.Done()
			defer func() { <-sem }()

			url, err := ensureGameIconLocal(id, true)
			if err != nil {
				steamLog().Debug("steam local game icon failed", slog.String("appId", id), slog.Any("err", err))
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

// downloadGameIcon fetches header art, the only artwork the CDN will serve from
// an app id alone. Steam's square icon lives at a per-app img_icon_url hash that
// nothing here stores, so this is deliberately the last resort below every local
// square asset, and the games view crops it to its square slot.
//
// It returns nil, nil when the CDN has no header art for this app. That is the
// ordinary outcome for tools, demos and delisted apps, so it stays at debug
// level rather than being reported as a failure.
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
		steamLog().Debug("steam game icon not on cdn", slog.String("appId", id), slog.Int("status", resp.StatusCode))
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
		steamLog().Debug("steam game icon cdn response is not a jpeg",
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
				steamLog().Debug("steam game icon warm failed", slog.String("appId", id), slog.Any("err", err))
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
