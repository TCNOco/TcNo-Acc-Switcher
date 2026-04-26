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
	var patterns []string
	for _, raw := range d.Extras.CachePaths {
		pattern, err := ResolveSafeDeletePattern(raw, ctx)
		if err != nil {
			return err
		}
		if pattern != "" {
			patterns = append(patterns, pattern)
		}
	}

	var errs []error
	for _, pattern := range patterns {
		for _, path := range ExpandDeletePatternMatches(pattern) {
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

func ResolveSafeDeletePattern(raw string, ctx PathTokenContext) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	expanded := ExpandPathTokens(raw, ctx)
	if hasPlaceholderToken(expanded) {
		return "", fmt.Errorf("unsafe cache path has unresolved placeholder: %s", raw)
	}
	expanded = filepath.Clean(strings.TrimSpace(expanded))
	if expanded == "" || expanded == "." || isVolumeRoot(expanded) {
		return "", fmt.Errorf("unsafe cache path resolves to root: %s", raw)
	}
	bases, err := cachePlaceholderBases(raw, ctx)
	if err != nil {
		return "", err
	}
	staticPrefix := filepath.Clean(staticCachePathPrefix(expanded))
	for _, base := range bases {
		if samePath(expanded, base) || samePath(staticPrefix, base) {
			return "", fmt.Errorf("unsafe cache path resolves to placeholder base: %s", raw)
		}
	}
	return expanded, nil
}

func ExpandDeletePatternMatches(path string) []string {
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

func hasPlaceholderToken(path string) bool {
	start := strings.Index(path, "%")
	if start < 0 {
		return false
	}
	return strings.Contains(path[start+1:], "%")
}

func cachePlaceholderBases(raw string, ctx PathTokenContext) ([]string, error) {
	home := os.Getenv("USERPROFILE")
	appData := os.Getenv("APPDATA")
	programData := os.Getenv("ProgramData")
	known := map[string]string{
		"%ProgramFiles%":         os.Getenv("ProgramFiles"),
		"%ProgramFiles(x86)%":    os.Getenv("ProgramFiles(x86)"),
		"%LocalAppData%":         os.Getenv("LocalAppData"),
		"%AppData%":              appData,
		"%UserProfile%":          home,
		"%USERPROFILE%":          home,
		"%Desktop%":              childPathIfBase(home, "Desktop"),
		"%Documents%":            childPathIfBase(home, "Documents"),
		"%Music%":                childPathIfBase(home, "Music"),
		"%Pictures%":             childPathIfBase(home, "Pictures"),
		"%Videos%":               childPathIfBase(home, "Videos"),
		"%ProgramData%":          programData,
		"%StartMenuAppData%":     childPathIfBase(appData, `Microsoft\Windows\Start Menu\Programs`),
		"%StartMenuProgramData%": childPathIfBase(programData, `Microsoft\Windows\Start Menu\Programs`),
		"%Platform_Folder%":      ctx.PlatformFolder,
	}

	var bases []string
	for token, base := range known {
		if !strings.Contains(raw, token) {
			continue
		}
		base = filepath.Clean(strings.TrimSpace(base))
		if base == "" || base == "." || isVolumeRoot(base) {
			return nil, fmt.Errorf("unsafe cache placeholder has invalid base: %s", token)
		}
		st, err := os.Stat(base)
		if err != nil {
			return nil, fmt.Errorf("cache placeholder base not found: %s", token)
		}
		if !st.IsDir() {
			return nil, fmt.Errorf("cache placeholder base is not a folder: %s", token)
		}
		bases = append(bases, base)
	}
	return bases, nil
}

func childPathIfBase(base, child string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	return filepath.Join(base, child)
}

func staticCachePathPrefix(path string) string {
	idx := strings.IndexAny(path, "*?")
	if idx < 0 {
		return path
	}
	prefix := path[:idx]
	sep := strings.LastIndexAny(prefix, `\/`)
	if sep < 0 {
		return "."
	}
	return prefix[:sep+1]
}

func clearCachePath(path string) error {
	if err := ValidateDeleteTargetPath(path); err != nil {
		return err
	}
	path = filepath.Clean(strings.TrimSpace(path))
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

func ValidateDeleteTargetPath(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || isVolumeRoot(path) {
		return fmt.Errorf("unsafe delete target resolves to root")
	}
	return nil
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
	return rest == "" || rest == string(filepath.Separator)
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
