package steam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"

	"github.com/ulikunitz/xz"
)

// InstalledGameInfo is one installed Steam app from libraryfolders / appmanifest scan.
type InstalledGameInfo struct {
	AppID string `json:"appId"`
	Name  string `json:"name"`
}

var reAppManifest = regexp.MustCompile(`(?i)^appmanifest_(\d+)\.acf$`)
var reQuotedPath = regexp.MustCompile(`"(?i)path"\s+"([^"]+)"`)
var reManifestName = regexp.MustCompile(`(?i)"name"\s+"([^"]*)"`)

// manifestNameMaxBytes bounds the read for one app's name. An appmanifest is a few
// KiB and the name sits near the top, so this is generous; the cap is here because
// the file is on disk Steam owns, not ours.
const manifestNameMaxBytes = 64 << 10

func steamAppsDirs(steamRoot string) ([]string, error) {
	steamRoot = filepath.Clean(steamRoot)
	out := []string{filepath.Join(steamRoot, "steamapps")}
	b, err := os.ReadFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"))
	if err == nil {
		for _, m := range reQuotedPath.FindAllStringSubmatch(string(b), -1) {
			if len(m) < 2 {
				continue
			}
			p := filepath.Clean(platformExpandPath(m[1]))
			if p == "" {
				continue
			}
			out = append(out, filepath.Join(p, "steamapps"))
		}
	}
	seen := map[string]struct{}{}
	var uniq []string
	for _, d := range out {
		d = filepath.Clean(d)
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		uniq = append(uniq, d)
	}
	return uniq, nil
}

func platformExpandPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return os.ExpandEnv(s)
}

// installedAppIDs maps each installed app id to the name Steam wrote in its
// appmanifest, or "" when the file has none.
//
// The manifest is the only name source that covers apps the store will not talk
// about at all - tools, redistributables, dedicated servers, hard-delisted games -
// because it is written by the client that installed them rather than by the store.
// The catalogue map has no entry for those at any point, so reading the file the
// scan already found is what keeps them from rendering as "App <id>".
func installedAppIDs(steamRoot string) (map[string]string, error) {
	dirs, err := steamAppsDirs(steamRoot)
	if err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for _, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			m := reAppManifest.FindStringSubmatch(e.Name())
			if len(m) < 2 {
				continue
			}
			// A second library folder listing the same app must not blank a name
			// already read from the first.
			name := manifestName(filepath.Join(dir, e.Name()))
			if existing, seen := ids[m[1]]; seen && name == "" {
				name = existing
			}
			ids[m[1]] = name
		}
	}
	return ids, nil
}

// manifestName reads the "name" field out of one appmanifest. An unreadable or
// nameless manifest is not an error: the caller falls back to the catalogue map.
func manifestName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, manifestNameMaxBytes))
	if err != nil {
		return ""
	}
	m := reManifestName.FindSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

func loginCacheSteamDir() (string, error) {
	return paths.LoginCacheDir("Steam")
}

func appIdsUserPath() (string, error) {
	base, err := loginCacheSteamDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "AppIdsUser.json"), nil
}

func loadAppNameMapFromDisk() (map[string]string, error) {
	p, err := appIdsUserPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	return parseAppNameMapJSON(b)
}

func saveAppNameMapToDisk(m map[string]string) error {
	p, err := appIdsUserPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, b, 0o644)
}

const (
	steamAppArrayMirrorXZURL    = "https://api.tcno.co/sw/SteamAppArrayXZ"
	steamAppArrayMirrorURL      = "https://api.tcno.co/sw/SteamAppArray"
	steamAppNameMapCacheTTL     = 24 * time.Hour
	steamAppNameMapFetchTimeout = 10 * time.Minute
	steamAppNameMapMaxJSONBytes = 32 << 20
	steamAppNameMapMaxXZBytes   = 8 << 20
)

type steamAppNameMapSource struct {
	url          string
	xzCompressed bool
}

var steamAppNameMapSources = []steamAppNameMapSource{
	{url: steamAppArrayMirrorXZURL, xzCompressed: true},
	{url: steamAppArrayMirrorURL, xzCompressed: false},
}

var (
	steamAppNameMapMu         sync.RWMutex
	steamAppNameMapMem        map[string]string
	steamAppNameMapRefreshMu  sync.Mutex
	steamAppNameMapRefreshing bool
)

func steamAppNameMapLooksValid(m map[string]string) bool {
	if len(m) == 0 {
		return false
	}
	for id, name := range m {
		if strings.TrimSpace(id) != "" && strings.TrimSpace(name) != "" {
			return true
		}
	}
	return false
}

func parseAppNameMapJSON(raw []byte) (map[string]string, error) {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if !steamAppNameMapLooksValid(m) {
		return nil, fmt.Errorf("steam app name map invalid")
	}
	return m, nil
}

func steamAppNameMapCacheModTime() (time.Time, bool) {
	cachePath, err := appIdsUserPath()
	if err != nil {
		return time.Time{}, false
	}
	st, err := os.Stat(cachePath)
	if err != nil || st.IsDir() {
		return time.Time{}, false
	}
	return st.ModTime(), true
}

func steamAppNameMapCacheExpired() bool {
	mt, ok := steamAppNameMapCacheModTime()
	if !ok {
		return true
	}
	return time.Since(mt) >= steamAppNameMapCacheTTL
}

func steamAppNameMapCacheAge() (time.Duration, bool) {
	mt, ok := steamAppNameMapCacheModTime()
	if !ok {
		return 0, false
	}
	return time.Since(mt), true
}

// setSteamAppNameMapMemory publishes m as the in-memory catalogue. It takes
// ownership: every producer builds a fresh map, and nothing mutates one after
// publishing, so the map is replaced wholesale rather than copied into.
func setSteamAppNameMapMemory(m map[string]string) {
	steamAppNameMapMu.Lock()
	steamAppNameMapMem = m
	steamAppNameMapMu.Unlock()
}

// getSteamAppNameMapCached returns the Steam app catalogue. The result is
// read-only and shared - do not write to it.
//
// It used to hand back a copy, which for the ~180k-entry catalogue is around
// 10MB and 180k hash inserts per call, on a map every caller only reads.
// appinfo.go's cache already returns its map uncopied for the same reason.
func getSteamAppNameMapCached() (map[string]string, error) {
	steamAppNameMapMu.RLock()
	if steamAppNameMapLooksValid(steamAppNameMapMem) {
		m := steamAppNameMapMem
		steamAppNameMapMu.RUnlock()
		return m, nil
	}
	steamAppNameMapMu.RUnlock()

	m, err := loadAppNameMapFromDisk()
	if err != nil {
		return nil, err
	}
	if !steamAppNameMapLooksValid(m) {
		return nil, fmt.Errorf("steam app name map cache invalid")
	}
	setSteamAppNameMapMemory(m)
	return m, nil
}

func downloadAndStoreAppNameMap(ctx context.Context, reason string) error {
	cachePath, err := appIdsUserPath()
	if err != nil {
		return err
	}

	steamLog().Info("steam app name map download started",
		slog.String("reason", reason),
		slog.String("cachePath", cachePath),
	)

	var lastErr error
	for _, source := range steamAppNameMapSources {
		steamLog().Info("steam app name map fetching",
			slog.String("url", source.url),
			slog.String("reason", reason),
			slog.Bool("xz", source.xzCompressed),
		)
		raw, compressedBytes, err := fetchSteamAppNameMapPayload(ctx, source)
		if err != nil {
			lastErr = err
			steamLog().Warn("steam app name map fetch failed",
				slog.String("url", source.url),
				slog.Bool("xz", source.xzCompressed),
				slog.Any("err", err),
			)
			continue
		}
		names, err := parseAppNameMapJSON(raw)
		if err != nil {
			lastErr = fmt.Errorf("invalid app name map payload from %s: %w", source.url, err)
			steamLog().Warn("steam app name map fetch invalid payload",
				slog.String("url", source.url),
				slog.Bool("xz", source.xzCompressed),
				slog.Int("compressedBytes", compressedBytes),
				slog.Int("jsonBytes", len(raw)),
				slog.Any("err", err),
			)
			continue
		}
		if err := saveAppNameMapToDisk(names); err != nil {
			return err
		}
		setSteamAppNameMapMemory(names)
		logArgs := []any{
			slog.String("reason", reason),
			slog.String("source", source.url),
			slog.Int("entries", len(names)),
			slog.Int("jsonBytes", len(raw)),
			slog.String("cachePath", cachePath),
		}
		if source.xzCompressed {
			logArgs = append(logArgs, slog.Int("compressedBytes", compressedBytes))
		}
		steamLog().Info("steam app name map refreshed", logArgs...)
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("steam app name map: empty")
}

func tryStartAppNameMapRefresh(reason string) {
	if appclient.IsOfflineMode() {
		steamLog().Info("steam app name map refresh skipped: offline mode", slog.String("reason", reason))
		return
	}
	steamAppNameMapRefreshMu.Lock()
	if steamAppNameMapRefreshing {
		steamAppNameMapRefreshMu.Unlock()
		steamLog().Debug("steam app name map refresh coalesced: already running", slog.String("reason", reason))
		return
	}
	steamAppNameMapRefreshing = true
	steamAppNameMapRefreshMu.Unlock()

	steamLog().Info("steam app name map background refresh queued", slog.String("reason", reason))

	go func() {
		defer crashlog.Capture()
		defer func() {
			steamAppNameMapRefreshMu.Lock()
			steamAppNameMapRefreshing = false
			steamAppNameMapRefreshMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), steamAppNameMapFetchTimeout)
		defer cancel()
		if err := downloadAndStoreAppNameMap(ctx, reason); err != nil {
			steamLog().Warn("steam app name map background refresh failed", slog.String("reason", reason), slog.Any("err", err))
		}
	}()
}

// StartSteamAppListMonitor warms the app name map from disk, refreshes immediately when
// AppIdsUser.json is older than 24 hours, and keeps refreshing every 24 hours.
func StartSteamAppListMonitor() {
	go runSteamAppNameMapMonitor()
}

func runSteamAppNameMapMonitor() {
	defer crashlog.Capture()
	cachePath, pathErr := appIdsUserPath()
	_, cacheErr := getSteamAppNameMapCached()

	logArgs := []any{
		slog.Duration("ttl", steamAppNameMapCacheTTL),
	}
	if pathErr == nil {
		logArgs = append(logArgs, slog.String("cachePath", cachePath))
	} else {
		logArgs = append(logArgs, slog.Any("cachePathErr", pathErr))
	}
	if cacheErr != nil {
		logArgs = append(logArgs, slog.String("cacheStatus", "missing"), slog.Any("cacheErr", cacheErr))
	} else if age, ok := steamAppNameMapCacheAge(); ok {
		logArgs = append(logArgs,
			slog.String("cacheStatus", "present"),
			slog.Duration("cacheAge", age),
			slog.Bool("cacheExpired", steamAppNameMapCacheExpired()),
		)
	} else {
		logArgs = append(logArgs, slog.String("cacheStatus", "missing"))
	}
	steamLog().Info("steam app name map monitor started", logArgs...)

	refreshIfStale := func() {
		if appclient.IsOfflineMode() {
			steamLog().Info("steam app name map refresh skipped: offline mode", slog.String("reason", "startup"))
			return
		}
		if steamAppNameMapCacheExpired() {
			tryStartAppNameMapRefresh("startup-stale")
			return
		}
		if age, ok := steamAppNameMapCacheAge(); ok {
			steamLog().Info("steam app name map refresh skipped: cache fresh",
				slog.String("reason", "startup"),
				slog.Duration("cacheAge", age),
			)
		}
	}
	refreshIfStale()

	ticker := time.NewTicker(steamAppNameMapCacheTTL)
	defer ticker.Stop()
	for range ticker.C {
		if appclient.IsOfflineMode() {
			steamLog().Info("steam app name map refresh skipped: offline mode", slog.String("reason", "scheduled"))
			continue
		}
		tryStartAppNameMapRefresh("scheduled")
	}
}

func fetchSteamAppNameMapPayload(ctx context.Context, source steamAppNameMapSource) ([]byte, int, error) {
	compressed, err := fetchSteamAppNameMapRaw(ctx, source)
	if err != nil {
		return nil, 0, err
	}
	if !source.xzCompressed {
		return compressed, len(compressed), nil
	}
	raw, err := decompressXZSteamAppNameMap(compressed)
	if err != nil {
		return nil, len(compressed), err
	}
	return raw, len(compressed), nil
}

func decompressXZSteamAppNameMap(compressed []byte) ([]byte, error) {
	r, err := xz.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("xz reader: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(r, steamAppNameMapMaxJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("xz decompress: %w", err)
	}
	return raw, nil
}

func fetchSteamAppNameMapRaw(ctx context.Context, source steamAppNameMapSource) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
	if err != nil {
		return nil, err
	}
	if source.xzCompressed {
		req.Header.Set("Accept", "application/x-xz")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher/3 (Steam app names; +https://github.com/TcNo-Acc-Switcher)")
	resp, err := appclient.Shared.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %d", source.url, resp.StatusCode)
	}
	limit := steamAppNameMapMaxJSONBytes
	if source.xzCompressed {
		limit = steamAppNameMapMaxXZBytes
	}
	return io.ReadAll(io.LimitReader(resp.Body, int64(limit)))
}

func ensureAppNameMap(ctx context.Context) (map[string]string, error) {
	if m, err := getSteamAppNameMapCached(); err == nil {
		if !appclient.IsOfflineMode() && steamAppNameMapCacheExpired() {
			tryStartAppNameMapRefresh("on-demand-stale")
		}
		return m, nil
	}

	if appclient.IsOfflineMode() {
		steamLog().Info("steam app name map download skipped: offline mode", slog.String("reason", "on-demand-missing"))
		return nil, fmt.Errorf("steam app name map: %w", appclient.ErrOfflineMode)
	}
	steamLog().Info("steam app name map cache missing; blocking download", slog.String("reason", "on-demand-missing"))
	if err := downloadAndStoreAppNameMap(ctx, "on-demand-missing"); err != nil {
		return nil, fmt.Errorf("steam app name map: %w", err)
	}
	return getSteamAppNameMapCached()
}

// resolveAppName is the shared fallback chain for an app with no manifest name:
// catalogue map, then the Steam client's appinfo cache, then the bare id.
func resolveAppName(names, local map[string]string, appID string) string {
	if nm := strings.TrimSpace(names[appID]); nm != "" {
		return nm
	}
	if nm := strings.TrimSpace(local[appID]); nm != "" {
		return nm
	}
	return "App " + appID
}

// BuildInstalledGamesList resolves names for installed ids using AppIdsUser.json.
func BuildInstalledGamesList(ctx context.Context, steamRoot string) ([]InstalledGameInfo, error) {
	installed, err := installedAppIDs(steamRoot)
	if err != nil {
		return nil, err
	}

	// Resolved after the scan so an unreadable Steam root never costs a blocking
	// app name map download.
	names, err := ensureAppNameMap(ctx)
	if err != nil {
		names = map[string]string{}
	}
	return namedInstalledGames(installed, names, appInfoNamesFn()), nil
}

// buildInstalledGamesListWithNames is BuildInstalledGamesList for a caller that
// already holds the name map. ensureAppNameMap hands back a full copy of the
// catalogue - ~180k entries, several megabytes - so a screen that needs names
// twice resolves once and threads the map through instead. The appinfo cache is
// threaded for the same reason.
func buildInstalledGamesListWithNames(steamRoot string, names, local map[string]string) ([]InstalledGameInfo, error) {
	installed, err := installedAppIDs(steamRoot)
	if err != nil {
		return nil, err
	}
	return namedInstalledGames(installed, names, local), nil
}

// namedInstalledGames resolves each installed app's display name.
//
// The appmanifest wins because it is the record Steam wrote for this exact install,
// so it is right even for an app the store has never listed. The catalogue map comes
// next, then the client's appinfo cache as the straggler net, and only then the id.
// ownedGameName applies the same order minus the manifest, which exists only for an
// app that is installed, so both halves of the games list read alike.
func namedInstalledGames(installed map[string]string, names, local map[string]string) []InstalledGameInfo {
	var list []InstalledGameInfo
	for id, manifestName := range installed {
		nm := strings.TrimSpace(manifestName)
		if nm == "" {
			nm = resolveAppName(names, local, id)
		}
		list = append(list, InstalledGameInfo{AppID: id, Name: nm})
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
	return list
}
