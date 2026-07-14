package platform

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultBgOpacity   float64 = 0.6
	defaultBgBlur      float64 = 6.0
	defaultBgAlignment         = "center"
	defaultBgFit               = "cover"
	bgSubDir                   = "backgrounds"
	bgMaxBytes         int64   = 50 << 20 // 50 MB
)

var bgAllowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
}

var bgResolveSourcePath = resolveBackgroundSourcePath

func bgSanitizeExt(fsPath string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(fsPath))
	return ext, bgAllowedExts[ext]
}

func bgDir(wwwroot string) string {
	return filepath.Join(wwwroot, bgSubDir)
}

func bgAppPrefix() string { return "app-bg" }

func bgPlatformPrefix(platformKey string) string {
	return "platform-" + sanitizePlatformSettingsFilePrefix(platformKey) + "-bg"
}

func bgClearFiles(dir, prefix string) error {
	for ext := range bgAllowedExts {
		p := filepath.Join(dir, prefix+ext)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func bgCopyFile(src, dst string) error {
	src = filepath.Clean(src)
	if strings.Contains(src, "\x00") {
		return errors.New("invalid path")
	}
	in, err := os.Open(src)
	if err != nil {
		resolved, ok := bgResolveSourcePath(src)
		if !ok || strings.EqualFold(filepath.Clean(resolved), src) {
			return err
		}
		in, err = os.Open(resolved)
		if err != nil {
			return err
		}
	}
	defer in.Close()

	st, err := in.Stat()
	if err != nil {
		return err
	}
	if st.Size() > bgMaxBytes {
		return fmt.Errorf("image too large (max %d MB)", bgMaxBytes>>20)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func bgInstallFile(src, dir, prefix, ext string) (string, error) {
	incoming := filepath.Join(dir, prefix+".incoming")
	_ = os.Remove(incoming)
	if err := bgCopyFile(src, incoming); err != nil {
		return "", err
	}
	defer os.Remove(incoming)

	if err := bgClearFiles(dir, prefix); err != nil {
		return "", err
	}
	dstName := prefix + ext
	if err := os.Rename(incoming, filepath.Join(dir, dstName)); err != nil {
		return "", err
	}
	return dstName, nil
}

func normalizeBackgroundAlignment(alignment string) string {
	value := strings.ToLower(strings.TrimSpace(alignment))
	switch value {
	case "left", "right", "top", "bottom":
		return value
	default:
		return defaultBgAlignment
	}
}

func normalizeBackgroundFit(fit string) string {
	value := strings.ToLower(strings.TrimSpace(fit))
	switch value {
	case "contain", "fill", "none", "scale-down":
		return value
	default:
		return defaultBgFit
	}
}

func buildAppBgInfo(img string, opacity, blur float64, alignment, fit string, override bool) AppBackgroundInfo {
	alignment = normalizeBackgroundAlignment(alignment)
	fit = normalizeBackgroundFit(fit)
	if img == "" {
		return AppBackgroundInfo{Opacity: defaultBgOpacity, Blur: defaultBgBlur, Alignment: alignment, Fit: fit, ThemeBgOverride: override}
	}
	op := opacity
	if op <= 0 {
		op = defaultBgOpacity
	}
	bl := blur
	if bl < 0 {
		bl = defaultBgBlur
	}
	return AppBackgroundInfo{
		HasImage:        true,
		ImageURL:        "/" + bgSubDir + "/" + img,
		Opacity:         op,
		Blur:            bl,
		Alignment:       alignment,
		Fit:             fit,
		ThemeBgOverride: override,
	}
}

//  ---------- App-wide background ----------

func (p *PlatformService) GetAppBackground() (AppBackgroundInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return AppBackgroundInfo{}, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return AppBackgroundInfo{}, err
	}
	return buildAppBgInfo(s.AppBgImage, s.AppBgOpacity, s.AppBgBlur, s.AppBgAlignment, s.AppBgFit, s.ThemeBgOverride), nil
}

func (p *PlatformService) SetAppBackground(imagePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	imagePath = strings.TrimSpace(imagePath)
	ext, ok := bgSanitizeExt(imagePath)
	if !ok {
		return fmt.Errorf("unsupported image type: %q", strings.ToLower(filepath.Ext(imagePath)))
	}

	wwwroot, err := WwwrootDir()
	if err != nil {
		return err
	}
	dir := bgDir(wwwroot)
	prefix := bgAppPrefix()

	dstName, err := bgInstallFile(imagePath, dir, prefix, ext)
	if err != nil {
		return err
	}

	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	s.AppBgImage = dstName
	s.ThemeBgOverride = true
	if s.AppBgOpacity <= 0 {
		s.AppBgOpacity = defaultBgOpacity
	}
	if s.AppBgBlur < 0 {
		s.AppBgBlur = defaultBgBlur
	}
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) ClearAppBackground() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	wwwroot, err := WwwrootDir()
	if err != nil {
		return err
	}
	_ = bgClearFiles(bgDir(wwwroot), bgAppPrefix())

	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	s.AppBgImage = ""
	s.AppBgOpacity = 0
	s.AppBgBlur = 0
	s.ThemeBgOverride = true
	return saveSettingsAtomic(exeDir, s)
}

// SetThemeBgOverride persists whether the user's background choice overrides the active
// theme's bundled background. Pass false to let the theme background show again.
func (p *PlatformService) SetThemeBgOverride(val bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	s.ThemeBgOverride = val
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetAppBackgroundOpacity(opacity float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if opacity < 0 {
		opacity = 0
	} else if opacity > 1 {
		opacity = 1
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	s.AppBgOpacity = opacity
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetAppBackgroundBlur(blur float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if blur < 0 {
		blur = 0
	} else if blur > 40 {
		blur = 40
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	s.AppBgBlur = blur
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetAppBackgroundAlignment(alignment string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	s.AppBgAlignment = normalizeBackgroundAlignment(alignment)
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetAppBackgroundFit(fit string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	s.AppBgFit = normalizeBackgroundFit(fit)
	return saveSettingsAtomic(exeDir, s)
}

// ---------- Per-platform background ----------

func (p *PlatformService) GetPlatformBackground(platformKey string) (AppBackgroundInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return AppBackgroundInfo{Opacity: defaultBgOpacity, Blur: defaultBgBlur, Alignment: defaultBgAlignment, Fit: defaultBgFit}, nil
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return AppBackgroundInfo{}, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return AppBackgroundInfo{}, err
	}
	ps, ok := s.PlatformBgs[platformKey]
	if !ok {
		return AppBackgroundInfo{Opacity: defaultBgOpacity, Blur: defaultBgBlur, Alignment: defaultBgAlignment, Fit: defaultBgFit}, nil
	}
	return buildAppBgInfo(ps.Image, ps.Opacity, ps.Blur, ps.Alignment, ps.Fit, false), nil
}

func (p *PlatformService) SetPlatformBackground(platformKey, imagePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("platformKey is required")
	}
	imagePath = strings.TrimSpace(imagePath)
	ext, ok := bgSanitizeExt(imagePath)
	if !ok {
		return fmt.Errorf("unsupported image type: %q", strings.ToLower(filepath.Ext(imagePath)))
	}

	wwwroot, err := WwwrootDir()
	if err != nil {
		return err
	}
	dir := bgDir(wwwroot)
	prefix := bgPlatformPrefix(platformKey)

	dstName, err := bgInstallFile(imagePath, dir, prefix, ext)
	if err != nil {
		return err
	}

	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	ps := s.PlatformBgs[platformKey]
	ps.Image = dstName
	if ps.Opacity <= 0 {
		ps.Opacity = defaultBgOpacity
	}
	if ps.Blur < 0 {
		ps.Blur = defaultBgBlur
	}
	if s.PlatformBgs == nil {
		s.PlatformBgs = make(map[string]PlatformBgSettings)
	}
	s.PlatformBgs[platformKey] = ps
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) ClearPlatformBackground(platformKey string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("platformKey is required")
	}

	wwwroot, err := WwwrootDir()
	if err != nil {
		return err
	}
	_ = bgClearFiles(bgDir(wwwroot), bgPlatformPrefix(platformKey))

	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	delete(s.PlatformBgs, platformKey)
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetPlatformBackgroundOpacity(platformKey string, opacity float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("platformKey is required")
	}
	if opacity < 0 {
		opacity = 0
	} else if opacity > 1 {
		opacity = 1
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	ps := s.PlatformBgs[platformKey]
	ps.Opacity = opacity
	if s.PlatformBgs == nil {
		s.PlatformBgs = make(map[string]PlatformBgSettings)
	}
	s.PlatformBgs[platformKey] = ps
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetPlatformBackgroundBlur(platformKey string, blur float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("platformKey is required")
	}
	if blur < 0 {
		blur = 0
	} else if blur > 40 {
		blur = 40
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	ps := s.PlatformBgs[platformKey]
	ps.Blur = blur
	if s.PlatformBgs == nil {
		s.PlatformBgs = make(map[string]PlatformBgSettings)
	}
	s.PlatformBgs[platformKey] = ps
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetPlatformBackgroundAlignment(platformKey, alignment string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("platformKey is required")
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	ps := s.PlatformBgs[platformKey]
	ps.Alignment = normalizeBackgroundAlignment(alignment)
	if s.PlatformBgs == nil {
		s.PlatformBgs = make(map[string]PlatformBgSettings)
	}
	s.PlatformBgs[platformKey] = ps
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) SetPlatformBackgroundFit(platformKey, fit string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("platformKey is required")
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	ps := s.PlatformBgs[platformKey]
	ps.Fit = normalizeBackgroundFit(fit)
	if s.PlatformBgs == nil {
		s.PlatformBgs = make(map[string]PlatformBgSettings)
	}
	s.PlatformBgs[platformKey] = ps
	return saveSettingsAtomic(exeDir, s)
}
