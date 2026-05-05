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
	vars := resolveDescriptorVariables(d, folder, ctx, "", false)
	switch key {
	case "ea desktop":
		cachePat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInProfileImageFile, vars, "", false)
		if strings.TrimSpace(cachePat) == "" {
			cachePat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\CEF\BrowserCache\EADesktop\Cache\Cache_Data\data_*`, vars, "", false)
		}
		userPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInUserId, vars, "", false)
		if strings.TrimSpace(userPat) == "" {
			userPat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\user_*.ini`, vars, "", false)
		}
		src, err := basicplatforms.EAImageSource(cachePat, userPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	case "rockstar":
		// Rockstar built-ins should resolve from descriptor paths (Documents), not platform exe folder context.
		dataPat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInProfileImageFile, vars, "", false)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%Documents%\Rockstar Games\Social Club\Launcher\Renderer\Default\Cache\Cache_Data\data_*`, vars, "", false)
		}
		src, err := basicplatforms.RockstarImageSource(dataPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	default:
		if built := descriptorBuiltInProfileSource(d, folder, ctx, vars, "", false); strings.TrimSpace(built.LocalPath) != "" || strings.TrimSpace(built.RemoteURL) != "" {
			return built, true, nil
		}
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
	vars := resolveDescriptorVariables(d, folder, ctx, "", false)
	switch key {
	case "ea desktop":
		dataPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInUsernameFile, vars, "", false)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\CEF\BrowserCache\EADesktop\Cache\Cache_Data\data_*`, vars, "", false)
		}
		userPat := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInUserId, vars, "", false)
		if strings.TrimSpace(userPat) == "" {
			userPat = builtInPatternPath(d, folder, ctx, `%LocalAppData%\Electronic Arts\EA Desktop\user_*.ini`, vars, "", false)
		}
		profileImageProviderLog.Debug("ea suggested-name patterns", "builtInUsernameFile", d.Extras.BuiltInUsernameFile, "builtInUserId", d.Extras.BuiltInUserId, "resolvedDataPattern", dataPat, "resolvedUserPattern", userPat)
		name, err := basicplatforms.EASuggestedName(dataPat, userPat)
		return strings.TrimSpace(name), true, err
	case "rockstar":
		// Rockstar built-ins should resolve from descriptor paths (Documents), not platform exe folder context.
		dataPat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInUsernameFile, vars, "", false)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%Documents%\Rockstar Games\Social Club\Launcher\Renderer\Default\Cache\Cache_Data\data_*`, vars, "", false)
		}
		profileImageProviderLog.Debug("rockstar suggested-name pattern", "builtInUsernameFile", d.Extras.BuiltInUsernameFile, "resolvedDataPattern", dataPat)
		name, err := basicplatforms.RockstarSuggestedName(dataPat)
		return strings.TrimSpace(name), true, err
	default:
		name := strings.TrimSpace(resolveDescriptorValue(d, d.Extras.BuiltInUsernameFile, folder, ctx, vars, "", false))
		if name == "" {
			return "", false, nil
		}
		return name, true, nil
	}
}

func platformProfileImagesSavedPerAccount(platformKey string) bool {
	k := strings.ToLower(strings.TrimSpace(platformKey))
	if k == "ea desktop" || k == "rockstar" {
		return true
	}
	d, _, err := readDescriptor(platformKey)
	if err != nil {
		return false
	}
	if strings.TrimSpace(d.Extras.BuiltInProfileImageFile) == "" || len(d.Extras.Variables) == 0 {
		return false
	}
	for _, raw := range d.Extras.Variables {
		if isLevelDBReference(raw) {
			return true
		}
	}
	return false
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
	vars := resolveDescriptorVariables(d, "", platform.PathTokenContext{}, root, true)
	key := strings.ToLower(strings.TrimSpace(platformKey))
	switch key {
	case "ea desktop":
		cachePat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInProfileImageFile, vars, root, true)
		if strings.TrimSpace(cachePat) == "" {
			cachePat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%LocalAppData%\Electronic Arts\EA Desktop\CEF\BrowserCache\EADesktop\Cache\Cache_Data\data_*`, vars, root, true)
		}
		userPat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInUserId, vars, root, true)
		if strings.TrimSpace(userPat) == "" {
			userPat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%LocalAppData%\Electronic Arts\EA Desktop\user_*.ini`, vars, root, true)
		}
		src, err := basicplatforms.EAImageSource(cachePat, userPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	case "rockstar":
		dataPat := builtInPatternPath(d, "", platform.PathTokenContext{}, d.Extras.BuiltInProfileImageFile, vars, root, true)
		if strings.TrimSpace(dataPat) == "" {
			dataPat = builtInPatternPath(d, "", platform.PathTokenContext{}, `%Documents%\Rockstar Games\Social Club\Launcher\Renderer\Default\Cache\Cache_Data\data_*`, vars, root, true)
		}
		src, err := basicplatforms.RockstarImageSource(dataPat)
		return profileImageSource{LocalPath: src.LocalPath, RemoteURL: src.RemoteURL}, true, err
	default:
		if built := descriptorBuiltInProfileSource(d, "", platform.PathTokenContext{}, vars, root, true); strings.TrimSpace(built.LocalPath) != "" || strings.TrimSpace(built.RemoteURL) != "" {
			return built, true, nil
		}
		return profileImageSource{}, true, nil
	}
}

func descriptorBuiltInProfileSource(d platform.Descriptor, folder string, ctx platform.PathTokenContext, vars map[string]string, accountCacheRoot string, saved bool) profileImageSource {
	p := builtInPatternPath(d, folder, ctx, d.Extras.BuiltInProfileImageFile, vars, accountCacheRoot, saved)
	p = strings.TrimSpace(p)
	if p == "" {
		return profileImageSource{}
	}
	if remoteProfilePicTemplate(p) {
		return profileImageSource{RemoteURL: p}
	}
	return profileImageSource{LocalPath: p}
}

func builtInPatternPath(d platform.Descriptor, folder string, ctx platform.PathTokenContext, builtInPattern string, vars map[string]string, accountCacheRoot string, saved bool) string {
	p := expandWithDescriptorVariables(builtInPattern, vars, folder, ctx)
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
	savedBuiltRoot := mapLivePathToSavedPath(d, folder, ctx, liveBuiltRoot, accountCacheRoot)
	if strings.TrimSpace(savedBuiltRoot) == "" {
		return ""
	}
	tail := strings.TrimPrefix(liveBuiltInPattern, liveBuiltRoot)
	return filepath.Clean(savedBuiltRoot) + tail
}

func mapLivePathToSavedPath(d platform.Descriptor, folder string, ctx platform.PathTokenContext, livePath, accountCacheRoot string) string {
	livePath = strings.TrimSpace(livePath)
	if livePath == "" || strings.TrimSpace(accountCacheRoot) == "" {
		return ""
	}
	for liveKey, cacheRel := range d.LoginFiles {
		liveExpanded := expandPlatformPath(liveKey, folder, ctx)
		liveLoginRoot := nonGlobPrefix(liveExpanded)
		if strings.TrimSpace(liveLoginRoot) == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(livePath), strings.ToLower(liveLoginRoot)) {
			continue
		}
		relFromLogin := strings.TrimPrefix(livePath, liveLoginRoot)
		relFromLogin = strings.TrimLeft(relFromLogin, `\/`)
		cacheRoot := filepath.Join(accountCacheRoot, filepath.FromSlash(cacheRel))
		return filepath.Join(cacheRoot, filepath.FromSlash(relFromLogin))
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
