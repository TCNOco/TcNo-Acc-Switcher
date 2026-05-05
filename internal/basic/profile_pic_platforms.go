package basic

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

type profileImageSource struct {
	LocalPath string
	RemoteURL string
}

type platformImageProvider func(folder string, ctx platform.PathTokenContext) (profileImageSource, error)

var platformImageProviders = map[string]platformImageProvider{
	"ea desktop": eaDesktopProfileImageSource,
}

func platformProfileImageSource(platformKey, folder string, ctx platform.PathTokenContext) (profileImageSource, bool, error) {
	p, ok := platformImageProviders[strings.ToLower(strings.TrimSpace(platformKey))]
	if !ok {
		return profileImageSource{}, false, nil
	}
	src, err := p(folder, ctx)
	return src, true, err
}

func platformHasProfileImageSource(platformKey string) bool {
	_, ok := platformImageProviders[strings.ToLower(strings.TrimSpace(platformKey))]
	return ok
}

func platformProfileImagesSavedPerAccount(platformKey string) bool {
	return strings.EqualFold(strings.TrimSpace(platformKey), "EA Desktop")
}

func platformProfileImageSourceFromSavedAccount(platformKey, accountName string) (profileImageSource, bool, error) {
	if !platformProfileImagesSavedPerAccount(platformKey) {
		return profileImageSource{}, false, nil
	}
	root, err := accountCacheDir(platformKey, accountName)
	if err != nil {
		return profileImageSource{}, false, err
	}
	if strings.EqualFold(strings.TrimSpace(platformKey), "EA Desktop") {
		src, err := eaDesktopProfileImageSourceFromSavedCache(root)
		return src, true, err
	}
	return profileImageSource{}, true, nil
}

func eaDesktopProfileImageSource(folder string, ctx platform.PathTokenContext) (profileImageSource, error) {
	userIniDir := platform.ExpandPathTokens(
		platform.ExpandWindowsPath("%LocalAppData%\\Electronic Arts\\EA Desktop"),
		ctx,
	)
	cacheDataDir := platform.ExpandPathTokens(
		platform.ExpandWindowsPath("%LocalAppData%\\Electronic Arts\\EA Desktop\\CEF\\BrowserCache\\EADesktop\\Cache\\Cache_Data"),
		ctx,
	)
	return eaDesktopProfileImageSourceFromDir(cacheDataDir, userIniDir)
}

func eaDesktopProfileImageSourceFromSavedCache(accountCacheRoot string) (profileImageSource, error) {
	userIniDir := filepath.Join(accountCacheRoot, "LocalAppData")
	cacheDataDir := filepath.Join(accountCacheRoot, "LocalAppData", "CEF", "BrowserCache", "EADesktop", "Cache", "Cache_Data")
	return eaDesktopProfileImageSourceFromDir(cacheDataDir, userIniDir)
}

func eaDesktopProfileImageSourceFromDir(cacheDataDir, userIniDir string) (profileImageSource, error) {
	userIDs := eaDesktopUserIDsByRecency(userIniDir)
	matches, err := filepath.Glob(filepath.Join(cacheDataDir, "data_*"))
	if err != nil || len(matches) == 0 {
		return profileImageSource{}, err
	}
	sort.Slice(matches, func(i, j int) bool {
		ist, ierr := os.Stat(matches[i])
		jst, jerr := os.Stat(matches[j])
		if ierr != nil || jerr != nil {
			return matches[i] > matches[j]
		}
		return ist.ModTime().After(jst.ModTime())
	})
	for _, f := range matches {
		st, err := os.Stat(f)
		if err != nil || st.IsDir() {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil || len(data) == 0 {
			continue
		}
		url := parseEAAvatarURLForUserIDs(data, userIDs)
		if strings.TrimSpace(url) != "" {
			return profileImageSource{RemoteURL: url}, nil
		}
	}
	return profileImageSource{}, nil
}

func parseEAAvatarURLForUserIDs(data []byte, userIDs []string) string {
	for _, uid := range userIDs {
		url := parseEAAvatarURLForUserID(data, uid)
		if url != "" {
			return url
		}
	}
	return parseEAAvatarURLForUserID(data, "")
}

func parseEAAvatarURLForUserID(data []byte, userID string) string {
	const marker = `{"data":{"me":`
	searchFrom := data
	if strings.TrimSpace(userID) != "" {
		userMarker := []byte(`{"data":{"me":{"id":"` + strings.TrimSpace(userID) + `"`)
		start := bytes.Index(data, userMarker)
		if start < 0 {
			return ""
		}
		searchFrom = data[start:]
	}
	start := bytes.Index(searchFrom, []byte(marker))
	if start < 0 {
		return ""
	}
	chunk := searchFrom[start:]
	largeIdx := bytes.Index(chunk, []byte(`"large"`))
	if largeIdx < 0 {
		return ""
	}
	chunk = chunk[largeIdx:]
	pathIdx := bytes.Index(chunk, []byte(`"path":"`))
	if pathIdx < 0 {
		return ""
	}
	chunk = chunk[pathIdx+len(`"path":"`):]
	end := bytes.IndexByte(chunk, '"')
	if end <= 0 {
		return ""
	}
	u := strings.TrimSpace(string(chunk[:end]))
	u = strings.ReplaceAll(u, `\/`, `/`)
	if strings.HasPrefix(strings.ToLower(u), "https://") || strings.HasPrefix(strings.ToLower(u), "http://") {
		return u
	}
	return ""
}

func eaDesktopUserIDsByRecency(userIniDir string) []string {
	files, err := filepath.Glob(filepath.Join(strings.TrimSpace(userIniDir), "user_*.ini"))
	if err != nil || len(files) == 0 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		ist, ierr := os.Stat(files[i])
		jst, jerr := os.Stat(files[j])
		if ierr != nil || jerr != nil {
			return files[i] > files[j]
		}
		return ist.ModTime().After(jst.ModTime())
	})
	seen := map[string]struct{}{}
	out := make([]string, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil || len(data) == 0 {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if !strings.HasPrefix(strings.ToLower(line), "user.userid=") {
				continue
			}
			id := strings.TrimSpace(line[len("user.userid="):])
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
			break
		}
	}
	return out
}
