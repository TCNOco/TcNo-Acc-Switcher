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
	"strconv"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"

	"github.com/tidwall/gjson"
	"github.com/ulikunitz/xz"
)

// InstalledGameInfo is one installed Steam app from libraryfolders / appmanifest scan.
type InstalledGameInfo struct {
	AppID string `json:"appId"`
	Name  string `json:"name"`
}

var reAppManifest = regexp.MustCompile(`(?i)^appmanifest_(\d+)\.acf$`)
var reQuotedPath = regexp.MustCompile(`"(?i)path"\s+"([^"]+)"`)

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

func installedAppIDs(steamRoot string) (map[string]struct{}, error) {
	dirs, err := steamAppsDirs(steamRoot)
	if err != nil {
		return nil, err
	}
	ids := map[string]struct{}{}
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
			ids[m[1]] = struct{}{}
		}
	}
	return ids, nil
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

func appIdsFullCachePath() (string, error) {
	base, err := loginCacheSteamDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "AppIdsFullListCache.json"), nil
}

func loadAppNameMap() (map[string]string, error) {
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
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func saveAppNameMap(m map[string]string) error {
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
	return os.WriteFile(p, b, 0o644)
}

const (
	steamAppListMirrorXZURL   = "https://api.tcno.co/sw/SteamAppListXZ"
	steamAppListValveURL      = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"
	steamAppListCacheTTL      = 24 * time.Hour
	steamAppListFetchTimeout  = 10 * time.Minute
	steamAppListMaxJSONBytes  = 120 << 20
	steamAppListMaxXZBytes    = 32 << 20
)

type steamAppListSource struct {
	url          string
	xzCompressed bool
}

var steamAppListSources = []steamAppListSource{
	{url: steamAppListMirrorXZURL, xzCompressed: true},
	{url: steamAppListValveURL, xzCompressed: false},
}

var (
	steamAppListMu       sync.RWMutex
	steamAppListData     []byte
	steamAppListRefreshMu sync.Mutex
	steamAppListRefreshing bool
)

func steamAppListJSONLooksValid(raw []byte) bool {
	if len(raw) < 50 {
		return false
	}
	if !gjson.GetBytes(raw, "applist.apps").Exists() {
		return false
	}
	// Cheap structural check (full list has many entries; avoids trusting tiny/corrupt files).
	return gjson.GetBytes(raw, "applist.apps.0.appid").Exists()
}

func normalizeAppListAppID(r gjson.Result) string {
	if !r.Exists() {
		return ""
	}
	switch r.Type {
	case gjson.String:
		return strings.TrimSpace(r.Str)
	case gjson.Number:
		f := r.Float()
		return strconv.FormatInt(int64(f), 10)
	default:
		s := strings.TrimSpace(r.String())
		if s != "" {
			return s
		}
		return strings.TrimSpace(strings.Trim(r.Raw, `"`))
	}
}

func steamAppListCacheModTime() (time.Time, bool) {
	cachePath, err := appIdsFullCachePath()
	if err != nil {
		return time.Time{}, false
	}
	st, err := os.Stat(cachePath)
	if err != nil || st.IsDir() {
		return time.Time{}, false
	}
	return st.ModTime(), true
}

func steamAppListCacheExpired() bool {
	mt, ok := steamAppListCacheModTime()
	if !ok {
		return true
	}
	return time.Since(mt) >= steamAppListCacheTTL
}

func steamAppListCacheAge() (time.Duration, bool) {
	mt, ok := steamAppListCacheModTime()
	if !ok {
		return 0, false
	}
	return time.Since(mt), true
}

func setSteamAppListMemory(raw []byte) {
	steamAppListMu.Lock()
	steamAppListData = raw
	steamAppListMu.Unlock()
}

func getSteamAppListCached() ([]byte, error) {
	steamAppListMu.RLock()
	if len(steamAppListData) > 0 && steamAppListJSONLooksValid(steamAppListData) {
		data := steamAppListData
		steamAppListMu.RUnlock()
		return data, nil
	}
	steamAppListMu.RUnlock()

	cachePath, err := appIdsFullCachePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	if !steamAppListJSONLooksValid(b) {
		return nil, fmt.Errorf("steam app list cache invalid")
	}
	setSteamAppListMemory(b)
	return b, nil
}

func downloadAndStoreSteamAppList(ctx context.Context, reason string) error {
	cachePath, err := appIdsFullCachePath()
	if err != nil {
		return err
	}

	steamLog.Info("steam app list download started",
		slog.String("reason", reason),
		slog.String("cachePath", cachePath),
	)

	var lastErr error
	for _, source := range steamAppListSources {
		steamLog.Info("steam app list fetching",
			slog.String("url", source.url),
			slog.String("reason", reason),
			slog.Bool("xz", source.xzCompressed),
		)
		raw, compressedBytes, err := fetchSteamAppListPayload(ctx, source)
		if err != nil {
			lastErr = err
			steamLog.Warn("steam app list fetch failed",
				slog.String("url", source.url),
				slog.Bool("xz", source.xzCompressed),
				slog.Any("err", err),
			)
			continue
		}
		if !steamAppListJSONLooksValid(raw) {
			lastErr = fmt.Errorf("invalid app list payload from %s", source.url)
			steamLog.Warn("steam app list fetch invalid payload",
				slog.String("url", source.url),
				slog.Bool("xz", source.xzCompressed),
				slog.Int("compressedBytes", compressedBytes),
				slog.Int("jsonBytes", len(raw)),
			)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return err
		}
		if err := fsutil.WriteFileAtomic(cachePath, raw, 0o644); err != nil {
			return err
		}
		setSteamAppListMemory(raw)
		logArgs := []any{
			slog.String("reason", reason),
			slog.String("source", source.url),
			slog.Int("jsonBytes", len(raw)),
			slog.String("cachePath", cachePath),
		}
		if source.xzCompressed {
			logArgs = append(logArgs, slog.Int("compressedBytes", compressedBytes))
		}
		steamLog.Info("steam app list refreshed", logArgs...)
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("steam app list: empty")
}

func tryStartSteamAppListRefresh(reason string) {
	if appclient.IsOfflineMode() {
		steamLog.Info("steam app list refresh skipped: offline mode", slog.String("reason", reason))
		return
	}
	steamAppListRefreshMu.Lock()
	if steamAppListRefreshing {
		steamAppListRefreshMu.Unlock()
		steamLog.Debug("steam app list refresh coalesced: already running", slog.String("reason", reason))
		return
	}
	steamAppListRefreshing = true
	steamAppListRefreshMu.Unlock()

	steamLog.Info("steam app list background refresh queued", slog.String("reason", reason))

	go func() {
		defer func() {
			steamAppListRefreshMu.Lock()
			steamAppListRefreshing = false
			steamAppListRefreshMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), steamAppListFetchTimeout)
		defer cancel()
		if err := downloadAndStoreSteamAppList(ctx, reason); err != nil {
			steamLog.Warn("steam app list background refresh failed", slog.String("reason", reason), slog.Any("err", err))
		}
	}()
}

// StartSteamAppListMonitor warms the in-memory app list from disk, refreshes immediately
// when the on-disk cache is older than 24 hours, and keeps refreshing every 24 hours.
func StartSteamAppListMonitor() {
	go runSteamAppListMonitor()
}

func runSteamAppListMonitor() {
	cachePath, pathErr := appIdsFullCachePath()
	_, cacheErr := getSteamAppListCached()

	logArgs := []any{
		slog.Duration("ttl", steamAppListCacheTTL),
	}
	if pathErr == nil {
		logArgs = append(logArgs, slog.String("cachePath", cachePath))
	} else {
		logArgs = append(logArgs, slog.Any("cachePathErr", pathErr))
	}
	if cacheErr != nil {
		logArgs = append(logArgs, slog.String("cacheStatus", "missing"), slog.Any("cacheErr", cacheErr))
	} else if age, ok := steamAppListCacheAge(); ok {
		logArgs = append(logArgs,
			slog.String("cacheStatus", "present"),
			slog.Duration("cacheAge", age),
			slog.Bool("cacheExpired", steamAppListCacheExpired()),
		)
	} else {
		logArgs = append(logArgs, slog.String("cacheStatus", "missing"))
	}
	steamLog.Info("steam app list monitor started", logArgs...)

	refreshIfStale := func() {
		if appclient.IsOfflineMode() {
			steamLog.Info("steam app list refresh skipped: offline mode", slog.String("reason", "startup"))
			return
		}
		if steamAppListCacheExpired() {
			tryStartSteamAppListRefresh("startup-stale")
			return
		}
		if age, ok := steamAppListCacheAge(); ok {
			steamLog.Info("steam app list refresh skipped: cache fresh",
				slog.String("reason", "startup"),
				slog.Duration("cacheAge", age),
			)
		}
	}
	refreshIfStale()

	ticker := time.NewTicker(steamAppListCacheTTL)
	defer ticker.Stop()
	for range ticker.C {
		if appclient.IsOfflineMode() {
			steamLog.Info("steam app list refresh skipped: offline mode", slog.String("reason", "scheduled"))
			continue
		}
		tryStartSteamAppListRefresh("scheduled")
	}
}

func fetchSteamAppListPayload(ctx context.Context, source steamAppListSource) ([]byte, int, error) {
	compressed, err := fetchSteamAppListRaw(ctx, source)
	if err != nil {
		return nil, 0, err
	}
	if !source.xzCompressed {
		return compressed, len(compressed), nil
	}
	raw, err := decompressXZSteamAppList(compressed)
	if err != nil {
		return nil, len(compressed), err
	}
	return raw, len(compressed), nil
}

func decompressXZSteamAppList(compressed []byte) ([]byte, error) {
	r, err := xz.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("xz reader: %w", err)
	}
	raw, err := io.ReadAll(io.LimitReader(r, steamAppListMaxJSONBytes))
	if err != nil {
		return nil, fmt.Errorf("xz decompress: %w", err)
	}
	return raw, nil
}

func fetchSteamAppListRaw(ctx context.Context, source steamAppListSource) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
	if err != nil {
		return nil, err
	}
	if source.xzCompressed {
		req.Header.Set("Accept", "application/x-xz")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher/3 (Steam app list; +https://github.com/TcNo-Acc-Switcher)")
	resp, err := appclient.Shared.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %d", source.url, resp.StatusCode)
	}
	limit := steamAppListMaxJSONBytes
	if source.xzCompressed {
		limit = steamAppListMaxXZBytes
	}
	return io.ReadAll(io.LimitReader(resp.Body, int64(limit)))
}

func ensureFullAppListCache(ctx context.Context) ([]byte, error) {
	if b, err := getSteamAppListCached(); err == nil {
		if !appclient.IsOfflineMode() && steamAppListCacheExpired() {
			tryStartSteamAppListRefresh("on-demand-stale")
		}
		return b, nil
	}

	if appclient.IsOfflineMode() {
		steamLog.Info("steam app list download skipped: offline mode", slog.String("reason", "on-demand-missing"))
		return nil, fmt.Errorf("steam app list: %w", appclient.ErrOfflineMode)
	}
	steamLog.Info("steam app list cache missing; blocking download", slog.String("reason", "on-demand-missing"))
	if err := downloadAndStoreSteamAppList(ctx, "on-demand-missing"); err != nil {
		return nil, fmt.Errorf("steam app list: %w", err)
	}
	return getSteamAppListCached()
}

// BuildInstalledGamesList resolves names for installed ids and writes AppIdsUser.json.
func BuildInstalledGamesList(ctx context.Context, steamRoot string) ([]InstalledGameInfo, error) {
	installed, err := installedAppIDs(steamRoot)
	if err != nil {
		return nil, err
	}
	names, err := loadAppNameMap()
	if err != nil {
		names = map[string]string{}
	}

	missingNames := false
	for id := range installed {
		if strings.TrimSpace(names[id]) == "" {
			missingNames = true
			break
		}
	}

	if missingNames {
		raw, err := ensureFullAppListCache(ctx)
		if err != nil {
			// proceed with numeric ids only
		} else {
			applist := gjson.GetBytes(raw, "applist.apps")
			if applist.Exists() {
				applist.ForEach(func(_, value gjson.Result) bool {
					appidStr := normalizeAppListAppID(value.Get("appid"))
					name := strings.TrimSpace(value.Get("name").String())
					if appidStr != "" && name != "" {
						names[appidStr] = name
					}
					return true
				})
			}
		}
		_ = saveAppNameMap(names)
	}

	var list []InstalledGameInfo
	for id := range installed {
		nm := strings.TrimSpace(names[id])
		if nm == "" {
			nm = "App " + id
		}
		list = append(list, InstalledGameInfo{AppID: id, Name: nm})
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
	return list, nil
}
