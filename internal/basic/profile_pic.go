package basic

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
)

func remoteProfilePicTemplate(tpl string) bool {
	t := strings.TrimSpace(strings.ToLower(tpl))
	return strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://")
}

func saveProfileImage(d platform.Descriptor, platformKey, folder, uid string, ctx platform.PathTokenContext) error {
	ctx.UniqueID = uid
	if strings.TrimSpace(d.UniqueIdFile) != "" {
		p := platform.ExpandPathTokens(platform.ExpandWindowsPath(d.UniqueIdFile), ctx)
		ctx.FileName = filepath.Base(p)
	}

	ex := d.Extras
	vars := resolveDescriptorVariables(d, folder, ctx, "", false)
	var src string
	var remoteURL string
	ps, err := platform.LoadPlatformSettings(platformKey)
	pullOnSwitch := err == nil && ps.PullAccountImagesOnSwitch
	if err != nil {
		pullOnSwitch = true
		ps = platform.DefaultPlatformSettings()
	}
	if !pullOnSwitch {
		slog.Debug("profile image skipped: pull disabled", "platform", platformKey, "uid", uid)
		return nil
	}

	if strings.TrimSpace(ex.ProfilePicFromFile) != "" && strings.TrimSpace(ex.ProfilePicRegex) != "" {
		if s, err := profilePicFromFile(ex.ProfilePicFromFile, ex.ProfilePicRegex, folder, ctx); err == nil && s != "" {
			src = s
		}
	}
	profilePicPath := expandDescriptorVariables(ex.ProfilePicPath, vars)
	if src == "" && remoteProfilePicTemplate(profilePicPath) {
		if strings.Contains(profilePicPath, "%LARGEST%") {
			return nil
		}
		url := expandPlatformPath(profilePicPath, folder, ctx)
		if !remoteProfilePicTemplate(url) {
			return nil
		}
		maxAge := ps.ProfileImageExpiryDays
		if maxAge <= 0 {
			maxAge = 7
		}
		queueProfileImageDownload(platformKey, uid, url, maxAge)
		return nil
	}
	if src == "" {
		if psrc, found, err := platformProfileImageSource(platformKey, folder, ctx); err == nil && found {
			src = strings.TrimSpace(psrc.LocalPath)
			remoteURL = strings.TrimSpace(psrc.RemoteURL)
		}
	}
	if src == "" && strings.TrimSpace(profilePicPath) != "" {
		if s, err := profilePicPathResolved(profilePicPath, folder, ctx); err == nil && s != "" {
			src = s
		}
	}
	if strings.TrimSpace(remoteURL) != "" {
		slog.Debug("profile image remote source queued", "platform", platformKey, "uid", uid, "url", remoteURL)
		queueProfileImageDownload(platformKey, uid, remoteURL, 0)
		return nil
	}
	if strings.TrimSpace(src) == "" {
		slog.Debug("profile image source not found", "platform", platformKey, "uid", uid)
		return nil
	}
	st, err := os.Stat(src)
	if err != nil || st.IsDir() {
		slog.Debug("profile image local source missing", "platform", platformKey, "uid", uid, "src", src, "err", err)
		return nil
	}
	slog.Debug("profile image local source queued", "platform", platformKey, "uid", uid, "src", src)
	queueProfileImageLocalCache(platformKey, uid, src)
	return nil
}

func queueProfileImageDownload(platformKey, uid, remoteURL string, maxAge int) {
	platformKey = strings.TrimSpace(platformKey)
	uid = strings.TrimSpace(uid)
	remoteURL = strings.TrimSpace(remoteURL)
	if platformKey == "" || uid == "" || remoteURL == "" {
		return
	}
	if appclient.IsOfflineMode() {
		slog.Debug("profile image download skipped: offline mode", "platform", platformKey, "uid", uid, "url", remoteURL)
		return
	}
	slog.Debug("profile image download queued", "platform", platformKey, "uid", uid, "url", remoteURL, "maxAgeDays", maxAge)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if res, err := profileimage.DownloadIfNeeded(ctx, appclient.Shared, platformKey, uid, remoteURL, maxAge); err != nil {
			slog.Debug("profile image download failed", "platform", platformKey, "uid", uid, "err", err)
			if cached, ok := profileimage.FindCached(platformKey, uid); ok {
				emitAccountImagePatch(AccountImagePatch{
					PlatformKey: platformKey,
					UniqueID:    uid,
					ImageURL:    cached,
				})
			}
		} else {
			slog.Debug("profile image download finished", "platform", platformKey, "uid", uid, "publicURL", res.PublicURL, "localPath", res.LocalPath)
			emitAccountImagePatch(AccountImagePatch{
				PlatformKey: platformKey,
				UniqueID:    uid,
				ImageURL:    res.PublicURL,
			})
		}
	}()
}

func queueProfileImageLocalCache(platformKey, uid, src string) {
	platformKey = strings.TrimSpace(platformKey)
	uid = strings.TrimSpace(uid)
	src = strings.TrimSpace(src)
	if platformKey == "" || uid == "" || src == "" {
		return
	}
	slog.Debug("profile image local cache queued", "platform", platformKey, "uid", uid, "src", src)
	go func() {
		if err := profileimage.CacheLocalFile(platformKey, uid, src); err != nil {
			slog.Debug("profile image local cache failed", "platform", platformKey, "uid", uid, "err", err)
			if cached, ok := profileimage.FindCached(platformKey, uid); ok {
				emitAccountImagePatch(AccountImagePatch{
					PlatformKey: platformKey,
					UniqueID:    uid,
					ImageURL:    cached,
				})
			}
		} else {
			slog.Debug("profile image local cache finished", "platform", platformKey, "uid", uid, "src", src)
			if cached, ok := profileimage.FindCached(platformKey, uid); ok {
				emitAccountImagePatch(AccountImagePatch{
					PlatformKey: platformKey,
					UniqueID:    uid,
					ImageURL:    cached,
				})
			}
		}
	}()
}

func profilePicPathResolved(tpl string, folder string, ctx platform.PathTokenContext) (string, error) {
	tpl = strings.TrimSpace(tpl)
	if tpl == "" {
		return "", nil
	}
	if remoteProfilePicTemplate(tpl) {
		if strings.Contains(tpl, "%LARGEST%") {
			return "", nil
		}
		return expandPlatformPath(tpl, folder, ctx), nil
	}
	if !strings.Contains(tpl, "%LARGEST%") {
		return expandPlatformPath(tpl, folder, ctx), nil
	}
	idx := strings.Index(tpl, "%LARGEST%")
	before, after := tpl[:idx], tpl[idx+len("%LARGEST%"):]
	beforeExp := expandPlatformPath(before, folder, ctx)
	dir := filepath.Dir(beforeExp)
	glob := filepath.Join(dir, "*"+after)
	matches, err := filepath.Glob(glob)
	if err != nil || len(matches) == 0 {
		return "", nil
	}
	var best string
	var bestSize int64
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil || st.IsDir() {
			continue
		}
		if st.Size() > bestSize {
			bestSize = st.Size()
			best = m
		}
	}
	if best == "" {
		return "", nil
	}
	stem := strings.TrimSuffix(filepath.Base(best), filepath.Ext(best))
	ctx2 := ctx
	ctx2.LargestPath = stem
	return expandPlatformPath(tpl, folder, ctx2), nil
}

func profilePicFromFile(pattern, regexName, folder string, ctx platform.PathTokenContext) (string, error) {
	pat := expandPlatformPath(pattern, folder, ctx)
	rx, err := platform.ExpandRegex(regexName)
	if err != nil {
		return "", err
	}
	if rx == nil {
		return "", nil
	}
	matches, err := filepath.Glob(pat)
	if err != nil {
		return "", err
	}
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := string(data)
		m := rx.FindStringSubmatch(text)
		if len(m) == 0 {
			continue
		}
		candidate := strings.TrimSpace(m[0])
		if len(m) > 1 {
			candidate = strings.TrimSpace(m[1])
		}
		if candidate == "" {
			continue
		}
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
	}
	return "", nil
}
