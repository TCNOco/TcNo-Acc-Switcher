package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/paths"

	"github.com/tidwall/gjson"
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
	steamAppListMirrorURL = "https://api.tcno.co/sw/SteamAppList"
	steamAppListValveURL  = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"
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

func fetchSteamAppListRaw(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher/3 (Steam app list; +https://github.com/TcNo-Acc-Switcher)")
	resp, err := appclient.Shared.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 120<<20))
}

func ensureFullAppListCache(ctx context.Context) ([]byte, error) {
	cachePath, err := appIdsFullCachePath()
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(cachePath); err == nil && steamAppListJSONLooksValid(b) {
		return b, nil
	}

	if appclient.IsOfflineMode() {
		return nil, fmt.Errorf("steam app list: %w", appclient.ErrOfflineMode)
	}

	var lastErr error
	for _, url := range []string{steamAppListMirrorURL, steamAppListValveURL} {
		raw, err := fetchSteamAppListRaw(ctx, url)
		if err != nil {
			lastErr = err
			continue
		}
		if !steamAppListJSONLooksValid(raw) {
			lastErr = fmt.Errorf("invalid app list payload from %s", url)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(cachePath, raw, 0o644); err != nil {
			return nil, err
		}
		return raw, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("steam app list: %w", lastErr)
	}
	return nil, fmt.Errorf("steam app list: empty")
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
						if _, ok := installed[appidStr]; ok {
							names[appidStr] = name
						}
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
