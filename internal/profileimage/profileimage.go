// Package profileimage downloads and caches account avatars under wwwroot/img/profiles/<platform>/.
package profileimage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

// ManualProfileMarkerSuffix is appended to the account avatar cache stem (same as image basename without ext).
const ManualProfileMarkerSuffix = ".manual_profile"

func manualMarkerPath(platformKey, avatarStem string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	avatarStem = strings.TrimSpace(avatarStem)
	if platformKey == "" || avatarStem == "" {
		return "", fmt.Errorf("empty platform or stem")
	}
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, avatarStem+ManualProfileMarkerSuffix), nil
}

// HasManualProfileMarker returns true if this account stem has a manual (user-set) avatar lock.
func HasManualProfileMarker(platformKey, avatarStemForMainAvatar string) bool {
	platformKey = strings.TrimSpace(platformKey)
	avatarStemForMainAvatar = strings.TrimSpace(avatarStemForMainAvatar)
	if platformKey == "" || avatarStemForMainAvatar == "" {
		return false
	}
	p, err := manualMarkerPath(platformKey, avatarStemForMainAvatar)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// WriteManualProfileMarker writes the sentinel beside cached profile images for the main avatar stem.
func WriteManualProfileMarker(platformKey, avatarStemForMainAvatar string) error {
	platformKey = strings.TrimSpace(platformKey)
	avatarStemForMainAvatar = strings.TrimSpace(avatarStemForMainAvatar)
	if platformKey == "" || avatarStemForMainAvatar == "" {
		return nil
	}
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p, err := manualMarkerPath(platformKey, avatarStemForMainAvatar)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p, nil, 0o644)
}

// ClearManualProfileMarker removes the manual avatar sentinel without deleting image bytes.
func ClearManualProfileMarker(platformKey, avatarStemForMainAvatar string) error {
	p, err := manualMarkerPath(platformKey, avatarStemForMainAvatar)
	if err != nil || p == "" {
		return err
	}
	return os.Remove(p)
}

// MainAvatarStemFromCacheStem maps "steamid_aux" filenames to the owning account id stem for Steam extras.
func MainAvatarStemFromCacheStem(cacheStem string) string {
	cacheStem = strings.TrimSpace(cacheStem)
	if cacheStem == "" {
		return ""
	}
	for _, suf := range []string{"_frame", "_nameplate", "_featuredbadge"} {
		if strings.HasSuffix(cacheStem, suf) {
			return strings.TrimSuffix(cacheStem, suf)
		}
	}
	return cacheStem
}

// DeleteAutomatedProfileCaches removes avatar cache files whose main account stem is not manual-locked,
// removes all auxiliary Steam assets (_frame etc.), always keeps "*.manual_profile" files.
func DeleteAutomatedProfileCaches(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil
	}
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	knownImgExt := map[string]struct{}{
		".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {},
		".gif": {}, ".webm": {}, ".mp4": {},
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ManualProfileMarkerSuffix) {
			continue
		}
		ext := filepath.Ext(name)
		if _, ok := knownImgExt[strings.ToLower(ext)]; !ok {
			_ = os.Remove(filepath.Join(dir, name))
			continue
		}
		stem := strings.TrimSuffix(name, ext)
		mainStem := MainAvatarStemFromCacheStem(stem)
		if stem != mainStem {
			_ = os.Remove(filepath.Join(dir, name))
			continue
		}
		if HasManualProfileMarker(platformKey, mainStem) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
	return nil
}

// PlatformFolder returns a filesystem-safe folder name (e.g. "steam").
func PlatformFolder(platformKey string) string {
	s := strings.TrimSpace(strings.ToLower(platformKey))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == ' ' || r == '/' || r == '\\':
			b.WriteRune('_')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "unknown"
	}
	return out
}

// ProfileDir is {dataRoot}/wwwroot/img/profiles/<platformFolder>/.
func ProfileDir(platformKey string) (string, error) {
	www, err := paths.WwwrootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(www, "img", "profiles", PlatformFolder(platformKey)), nil
}

// PublicPath returns the URL path /img/profiles/<platform>/<id>.<ext> (leading slash).
func PublicPath(platformKey, accountID, ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	return fmt.Sprintf("/img/profiles/%s/%s.%s", PlatformFolder(platformKey), accountID, ext)
}

// CachedFilePath returns the on-disk path to an existing cached image, if any.
func CachedFilePath(platformKey, accountID string) (string, bool) {
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return "", false
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif", "webm", "mp4"} {
		full := filepath.Join(dir, accountID+"."+ext)
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			return full, true
		}
	}
	return "", false
}

// CacheLocalFile copies src into the profile image cache for automated pipelines (saved-account cache, etc.).
// If the user locked a manual avatar for this account, returns nil without reading src or changing disk — avoids
// a queued goroutine overwriting a user-chosen image (Rockstar/EA and similar).
func CacheLocalFile(platformKey, accountID, src string) error {
	return writeLocalProfileCache(platformKey, accountID, src, false)
}

// CacheLocalFileForUser copies src into the cache for user-initiated picks; overwrites any existing cached avatar and manual lock handling is done by the caller (marker write after success).
func CacheLocalFileForUser(platformKey, accountID, src string) error {
	return writeLocalProfileCache(platformKey, accountID, src, true)
}

func writeLocalProfileCache(platformKey, accountID, src string, userPick bool) error {
	platformKey = strings.TrimSpace(platformKey)
	accountID = strings.TrimSpace(accountID)
	src = strings.TrimSpace(src)
	if platformKey == "" || accountID == "" || src == "" {
		return nil
	}
	if !userPick && HasManualProfileMarker(platformKey, accountID) {
		slog.Debug("profileimage cache local skipped: manual avatar", "platform", platformKey, "accountID", accountID)
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read profile image %s: %w", src, err)
	}
	if userPick {
		_ = DeleteCachedImageFilesOnly(platformKey, accountID)
	} else {
		_ = DeleteCached(platformKey, accountID)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(src)), ".")
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "webm", "mp4":
	default:
		ext = "jpg"
	}
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir profile cache %s: %w", dir, err)
	}
	dest := filepath.Join(dir, accountID+"."+ext)
	if err := fsutil.WriteFileAtomic(dest, data, 0o644); err != nil {
		return fmt.Errorf("write profile cache %s: %w", dest, err)
	}
	return nil
}

// DeleteCachedImageFilesOnly removes on-disk avatar variants for stem; does not remove .manual_profile.
// Used before writing a user-picked avatar so concurrent refresh cannot treat the account as non-manual
// during a DeleteCached()+write window (Rockstar/Ubisoft/etc. aggressively re-copy launcher avatars).
func DeleteCachedImageFilesOnly(platformKey, accountID string) error {
	platformKey = strings.TrimSpace(platformKey)
	accountID = strings.TrimSpace(accountID)
	if platformKey == "" || accountID == "" {
		return nil
	}
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif", "webm", "mp4"} {
		p := filepath.Join(dir, accountID+"."+ext)
		_ = os.Remove(p)
	}
	return nil
}

// DeleteCached removes cached profile images for an account (any known extension).
func DeleteCached(platformKey, accountID string) error {
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif", "webm", "mp4"} {
		p := filepath.Join(dir, accountID+"."+ext)
		_ = os.Remove(p)
	}
	_ = ClearManualProfileMarker(platformKey, accountID)
	return nil
}

// DeletePlatformCached removes all cached profile images for a platform.
func DeletePlatformCached(platformKey string) error {
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// FindCached returns the public URL and true if a cached image exists for this account (any known ext).
func FindCached(platformKey, accountID string) (publicURL string, ok bool) {
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return "", false
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif", "webm", "mp4"} {
		full := filepath.Join(dir, accountID+"."+ext)
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			return PublicPath(platformKey, accountID, ext), true
		}
	}
	return "", false
}

// FileOlderThanDays returns true if path exists and mod time is older than days (or file missing -> false).
func FileOlderThanDays(path string, days int) bool {
	if days <= 0 {
		return true
	}
	st, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(st.ModTime()) > time.Duration(days)*24*time.Hour
}

// extFromContentType maps common image Content-Type to file extension (no dot).
func extFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	switch ct {
	case "image/jpeg":
		return "jpg"
	case "image/jpg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "video/webm":
		return "webm"
	case "video/mp4":
		return "mp4"
	default:
		return ""
	}
}

func extFromURL(u string) string {
	base := u
	if i := strings.LastIndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(base)), ".")
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "webm", "mp4":
		return ext
	default:
		return ""
	}
}

// DownloadResult is returned after a successful cache write.
type DownloadResult struct {
	PublicURL string
	LocalPath string
}

// DownloadIfNeeded GETs imageSourceURL and writes wwwroot/img/profiles/<platform>/<id>.<ext>.
// Skips download if a fresh cached file exists (not older than maxAgeDays).
func DownloadIfNeeded(ctx context.Context, client *http.Client, platformKey, accountID, imageSourceURL string, maxAgeDays int) (*DownloadResult, error) {
	if strings.TrimSpace(imageSourceURL) == "" {
		return nil, fmt.Errorf("empty image URL")
	}
	// Manual override: never replace via remote download for the main avatar id when locked.
	mainStem := MainAvatarStemFromCacheStem(strings.TrimSpace(accountID))
	if mainStem != "" && accountID == mainStem && HasManualProfileMarker(platformKey, mainStem) {
		if cachedURL, hit := FindCached(platformKey, accountID); hit {
			if local, ok := CachedFilePath(platformKey, accountID); ok {
				slog.Debug("profileimage download skipped: manual avatar", "platform", platformKey, "accountID", accountID)
				return &DownloadResult{PublicURL: cachedURL, LocalPath: local}, nil
			}
		}
		return nil, fmt.Errorf("manual profile marker without cached file")
	}
	slog.Debug("profileimage download begin", "platform", platformKey, "accountID", accountID, "url", imageSourceURL, "maxAgeDays", maxAgeDays)
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	// If a cached file exists and is still fresh, return without downloading.
	var existingPath, existingExt string
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif", "webm", "mp4"} {
		p := filepath.Join(dir, accountID+"."+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			existingPath, existingExt = p, ext
			break
		}
	}
	if existingPath != "" && !FileOlderThanDays(existingPath, maxAgeDays) {
		slog.Debug("profileimage download skipped: fresh cache", "platform", platformKey, "accountID", accountID, "path", existingPath, "ext", existingExt)
		return &DownloadResult{
			PublicURL: PublicPath(platformKey, accountID, existingExt),
			LocalPath: existingPath,
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageSourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TcNo Account Switcher")

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("profileimage download http failed", "platform", platformKey, "accountID", accountID, "url", imageSourceURL, "err", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Debug("profileimage download non-2xx", "platform", platformKey, "accountID", accountID, "status", resp.StatusCode, "url", imageSourceURL)
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	ext := extFromContentType(resp.Header.Get("Content-Type"))
	if ext == "" {
		ext = extFromURL(imageSourceURL)
	}
	if ext == "" {
		ext = "jpg"
	}

	maxBody := int64(20 << 20)
	if ext == "webm" || ext == "mp4" {
		maxBody = 80 << 20
	}

	dest := filepath.Join(dir, accountID+"."+ext)
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if err := fsutil.WriteFileAtomic(dest, data, 0o644); err != nil {
		return nil, err
	}
	slog.Debug("profileimage download saved", "platform", platformKey, "accountID", accountID, "dest", dest, "bytes", len(data), "ext", ext)

	return &DownloadResult{
		PublicURL: PublicPath(platformKey, accountID, ext),
		LocalPath: dest,
	}, nil
}
