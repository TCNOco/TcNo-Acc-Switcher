// Package profileimage downloads and caches account avatars under wwwroot/img/profiles/<platform>/.
package profileimage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

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
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif"} {
		full := filepath.Join(dir, accountID+"."+ext)
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			return full, true
		}
	}
	return "", false
}

// CacheLocalFile copies src into the profile image cache (overwrites existing).
func CacheLocalFile(platformKey, accountID, src string) error {
	platformKey = strings.TrimSpace(platformKey)
	accountID = strings.TrimSpace(accountID)
	src = strings.TrimSpace(src)
	if platformKey == "" || accountID == "" || src == "" {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = DeleteCached(platformKey, accountID)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(src)), ".")
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif":
	default:
		ext = "jpg"
	}
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, accountID+"."+ext)
	return fsutil.WriteFileAtomic(dest, data, 0o644)
}

// DeleteCached removes cached profile images for an account (any known extension).
func DeleteCached(platformKey, accountID string) error {
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return err
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif"} {
		p := filepath.Join(dir, accountID+"."+ext)
		_ = os.Remove(p)
	}
	return nil
}

// FindCached returns the public URL and true if a cached image exists for this account (any known ext).
func FindCached(platformKey, accountID string) (publicURL string, ok bool) {
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return "", false
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif"} {
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
	case "jpg", "jpeg", "png", "webp", "gif":
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
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	// If a cached file exists and is still fresh, return without downloading.
	var existingPath, existingExt string
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif"} {
		p := filepath.Join(dir, accountID+"."+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			existingPath, existingExt = p, ext
			break
		}
	}
	if existingPath != "" && !FileOlderThanDays(existingPath, maxAgeDays) {
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
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	ext := extFromContentType(resp.Header.Get("Content-Type"))
	if ext == "" {
		ext = extFromURL(imageSourceURL)
	}
	if ext == "" {
		ext = "jpg"
	}

	dest := filepath.Join(dir, accountID+"."+ext)
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	if err := fsutil.WriteFileAtomic(dest, data, 0o644); err != nil {
		return nil, err
	}

	return &DownloadResult{
		PublicURL: PublicPath(platformKey, accountID, ext),
		LocalPath: dest,
	}, nil
}
