package basic

import (
	"log/slog"
	"path/filepath"
	"strings"

	basicplatforms "TcNo-Acc-Switcher/internal/basic/platforms"
	"TcNo-Acc-Switcher/internal/platform"
)

type profileImageSource struct {
	LocalPath string
	RemoteURL string
}

var profileImageProviderLog = slog.Default().With("component", "profile-image-provider")

func platformProfileImageSource(platformKey, folder string, ctx platform.PathTokenContext) (profileImageSource, bool, error) {
	key := strings.ToLower(strings.TrimSpace(platformKey))
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return profileImageSource{}, false, err
	}
	switch key {
	case "ea desktop":
		cachePat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInProfileImageFile, "", false)
		if strings.TrimSpace(cachePat) == "" {
			cachePat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\CEF\BrowserCache\EADesktop\Cache\Cache_Data\data_*`, "", false)
		}
		userPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInUserId, "", false)
		if strings.TrimSpace(userPat) == "" {
			userPat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\user_*.ini`, "", false)
		}
		src, err := basicplatforms.EAImageSource(cachePat, userPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	case "rockstar":
		dataPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInProfileImageFile, "", false)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, folder, ctx, `%Documents%\Rockstar Games\Social Club\Launcher\Renderer\Default\Cache\Cache_Data\data_*`, "", false)
		}
		src, err := basicplatforms.RockstarImageSource(dataPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	default:
		return profileImageSource{}, false, nil
	}
}

func platformHasProfileImageSource(platformKey string) bool {
	switch strings.ToLower(strings.TrimSpace(platformKey)) {
	case "ea desktop", "rockstar":
		return true
	default:
		return false
	}
}

func platformSuggestedSaveName(platformKey, folder string, ctx platform.PathTokenContext) (string, bool, error) {
	key := strings.ToLower(strings.TrimSpace(platformKey))
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return "", false, err
	}
	switch key {
	case "ea desktop":
		dataPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInUsernameFile, "", false)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\CEF\BrowserCache\EADesktop\Cache\Cache_Data\data_*`, "", false)
		}
		userPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInUserId, "", false)
		if strings.TrimSpace(userPat) == "" {
			userPat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\user_*.ini`, "", false)
		}
		name, err := basicplatforms.EASuggestedName(dataPat, userPat)
		return strings.TrimSpace(name), true, err
	case "rockstar":
		dataPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInUsernameFile, "", false)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, folder, ctx, `%Documents%\Rockstar Games\Social Club\Launcher\Renderer\Default\Cache\Cache_Data\data_*`, "", false)
		}
		name, err := basicplatforms.RockstarSuggestedName(dataPat)
		return strings.TrimSpace(name), true, err
	default:
		return "", false, nil
	}
}

func platformProfileImagesSavedPerAccount(platformKey string) bool {
	k := strings.ToLower(strings.TrimSpace(platformKey))
	return k == "ea desktop" || k == "rockstar"
}

func platformProfileImageSourceFromSavedAccount(platformKey, accountName string) (profileImageSource, bool, error) {
	if !platformProfileImagesSavedPerAccount(platformKey) {
		return profileImageSource{}, false, nil
	}
	root, err := accountCacheDir(platformKey, accountName)
	if err != nil {
		return profileImageSource{}, false, err
	}
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return profileImageSource{}, false, err
	}
	key := strings.ToLower(strings.TrimSpace(platformKey))
	switch key {
	case "ea desktop":
		cachePat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInProfileImageFile, root, true)
		if strings.TrimSpace(cachePat) == "" {
			cachePat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%LocalAppData%\Electronic Arts\EA Desktop\CEF\BrowserCache\EADesktop\Cache\Cache_Data\data_*`, root, true)
		}
		userPat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInUserId, root, true)
		if strings.TrimSpace(userPat) == "" {
			userPat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%LocalAppData%\Electronic Arts\EA Desktop\user_*.ini`, root, true)
		}
		src, err := basicplatforms.EAImageSource(cachePat, userPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	case "rockstar":
		dataPat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInProfileImageFile, root, true)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%Documents%\Rockstar Games\Social Club\Launcher\Renderer\Default\Cache\Cache_Data\data_*`, root, true)
		}
		src, err := basicplatforms.RockstarImageSource(dataPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	default:
		return profileImageSource{}, true, nil
	}
}

func builtInPatternPath(d platform.Descriptor, folder string, ctx platform.PathTokenContext, builtInPattern, accountCacheRoot string, saved bool) string {
	p := platform.ExpandPathTokens(platform.ExpandWindowsPath(builtInPattern), ctx)
	if !saved || strings.TrimSpace(accountCacheRoot) == "" {
		return p
	}
	mapped := mapLiveBuiltInToSavedPattern(d, folder, ctx, p, accountCacheRoot)
	if strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return p
}

func mapLiveBuiltInToSavedPattern(d platform.Descriptor, folder string, ctx platform.PathTokenContext, liveBuiltInPattern, accountCacheRoot string) string {
	liveBuiltRoot := nonGlobPrefix(liveBuiltInPattern)
	if strings.TrimSpace(liveBuiltRoot) == "" {
		return ""
	}
	for liveKey, cacheRel := range d.LoginFiles {
		liveExpanded := expandPlatformPath(liveKey, folder, ctx)
		liveLoginRoot := nonGlobPrefix(liveExpanded)
		if strings.TrimSpace(liveLoginRoot) == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(liveBuiltRoot), strings.ToLower(liveLoginRoot)) {
			continue
		}
		relFromLogin := strings.TrimPrefix(liveBuiltRoot, liveLoginRoot)
		relFromLogin = strings.TrimLeft(relFromLogin, `\/`)
		cacheRoot := filepath.Join(accountCacheRoot, filepath.FromSlash(cacheRel))
		savedBuiltRoot := filepath.Join(cacheRoot, filepath.FromSlash(relFromLogin))
		tail := strings.TrimPrefix(liveBuiltInPattern, liveBuiltRoot)
		return filepath.Clean(savedBuiltRoot) + tail
	}
	return ""
}

func nonGlobPrefix(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	i := strings.IndexAny(path, "*?[")
	if i < 0 {
		return path
	}
	prefix := path[:i]
	if strings.HasSuffix(prefix, `\`) || strings.HasSuffix(prefix, `/`) {
		return prefix
	}
	return filepath.Dir(prefix)
}
