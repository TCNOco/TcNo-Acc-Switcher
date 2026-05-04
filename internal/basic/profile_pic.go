package basic

import (
	"context"
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
	var src string

	if strings.TrimSpace(ex.ProfilePicFromFile) != "" && strings.TrimSpace(ex.ProfilePicRegex) != "" {
		if s, err := profilePicFromFile(ex.ProfilePicFromFile, ex.ProfilePicRegex, folder, ctx); err == nil && s != "" {
			src = s
		}
	}
	if src == "" && remoteProfilePicTemplate(ex.ProfilePicPath) {
		if strings.Contains(ex.ProfilePicPath, "%LARGEST%") {
			return nil
		}
		url := expandPlatformPath(ex.ProfilePicPath, folder, ctx)
		if !remoteProfilePicTemplate(url) {
			return nil
		}
		ps, err := platform.LoadPlatformSettings(platformKey)
		if err != nil {
			return nil
		}
		maxAge := ps.ProfileImageExpiryDays
		if maxAge <= 0 {
			maxAge = 7
		}
		dctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_, _ = profileimage.DownloadIfNeeded(dctx, appclient.Shared, platformKey, uid, url, maxAge)
		return nil
	}
	if src == "" && strings.TrimSpace(ex.ProfilePicPath) != "" {
		if s, err := profilePicPathResolved(ex.ProfilePicPath, folder, ctx); err == nil && s != "" {
			src = s
		}
	}
	if strings.TrimSpace(src) == "" {
		return nil
	}
	st, err := os.Stat(src)
	if err != nil || st.IsDir() {
		return nil
	}
	return profileimage.CacheLocalFile(platformKey, uid, src)
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
