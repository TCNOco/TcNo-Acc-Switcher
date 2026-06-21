// Package gamestatsimage caches remote game-stat assets under wwwroot/img/<subdir>/.
package gamestatsimage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
)

const DefaultMaxAgeDays = 7

const maxImageBytes = 2 << 20 // 2 MiB

// FilenameFromURL returns the basename from imageURL (e.g. platinum4.png).
func FilenameFromURL(imageURL string) (string, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return "", fmt.Errorf("empty image URL")
	}
	base := imageURL
	if i := strings.LastIndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	name := filepath.Base(base)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid image filename in URL")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid image filename in URL")
	}
	return name, nil
}

func cacheDir(subdir string) (string, error) {
	subdir = strings.Trim(strings.ReplaceAll(subdir, "\\", "/"), "/")
	if subdir == "" {
		return "", fmt.Errorf("empty cache subdir")
	}
	www, err := paths.WwwrootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(www, "img", filepath.FromSlash(subdir)), nil
}

// PublicURL returns the web path served from wwwroot (no leading slash), e.g. img/gs/apex/platinum4.png.
func PublicURL(subdir, filename string) string {
	subdir = strings.Trim(strings.ReplaceAll(subdir, "\\", "/"), "/")
	filename = strings.TrimSpace(filename)
	return "img/" + subdir + "/" + filename
}

// DownloadIfNeeded GETs imageURL and writes wwwroot/img/<subdir>/<filename>.
// Skips download when a fresh cached file exists (not older than maxAgeDays).
func DownloadIfNeeded(ctx context.Context, client *http.Client, subdir, imageURL string, maxAgeDays int) (string, error) {
	filename, err := FilenameFromURL(imageURL)
	if err != nil {
		return "", err
	}
	dir, err := cacheDir(subdir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, filename)
	if st, statErr := os.Stat(dest); statErr == nil && !st.IsDir() && !profileimage.FileOlderThanDays(dest, maxAgeDays) {
		return PublicURL(subdir, filename), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("image HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("empty image body")
	}
	if err := fsutil.WriteFileAtomic(dest, raw, 0o644); err != nil {
		return "", err
	}
	return PublicURL(subdir, filename), nil
}
