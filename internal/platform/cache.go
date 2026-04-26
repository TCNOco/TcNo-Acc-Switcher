package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (p *PlatformService) HasPlatformCachePaths(platformKey string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	d, err := p.loadDescriptorUnlocked(platformKey)
	if err != nil {
		return false, err
	}
	for _, path := range d.Extras.CachePaths {
		if strings.TrimSpace(path) != "" {
			return true, nil
		}
	}
	return false, nil
}

func (p *PlatformService) ClearPlatformCache(platformKey string) error {
	p.mu.Lock()
	d, err := p.loadDescriptorUnlocked(platformKey)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	folder, err := p.getPlatformInstallFolderUnlocked(platformKey)
	p.mu.Unlock()
	if err != nil {
		return err
	}

	ctx := PathTokenContext{PlatformFolder: folder}
	var errs []error
	for _, raw := range d.Extras.CachePaths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		expanded := ExpandPathTokens(raw, ctx)
		for _, path := range expandCachePathMatches(expanded) {
			if err := clearCachePath(path); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (p *PlatformService) loadDescriptorUnlocked(platformKey string) (Descriptor, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return Descriptor{}, errors.New("empty platform")
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return Descriptor{}, err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return Descriptor{}, err
	}
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
	if err != nil {
		return Descriptor{}, err
	}
	return ParseDescriptor(raw, platformKey)
}

func expandCachePathMatches(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if !hasGlobMeta(path) {
		return []string{filepath.Clean(path)}
	}
	matches, err := filepath.Glob(path)
	if err != nil || len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, filepath.Clean(match))
	}
	return out
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?")
}

func clearCachePath(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || isVolumeRoot(path) {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !st.IsDir() {
		return os.Remove(path)
	}
	return clearDirectoryContents(path)
}

func clearDirectoryContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func isVolumeRoot(path string) bool {
	vol := filepath.VolumeName(path)
	if vol == "" {
		return path == string(filepath.Separator)
	}
	rest := strings.TrimPrefix(path, vol)
	return rest == string(filepath.Separator)
}
