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
	defaultBgOpacity float64 = 0.6
	defaultBgBlur    float64 = 6.0
	bgSubDir                 = "backgrounds"
	bgMaxBytes       int64   = 50 << 20 // 50 MB
)

var bgAllowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
}

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
		return err
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

func buildAppBgInfo(img string, opacity, blur float64) AppBackgroundInfo {
	if img == "" {
		return AppBackgroundInfo{Opacity: defaultBgOpacity, Blur: defaultBgBlur}
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
		HasImage: true,
		ImageURL: "/" + bgSubDir + "/" + img,
		Opacity:  op,
		Blur:     bl,
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
	return buildAppBgInfo(s.AppBgImage, s.AppBgOpacity, s.AppBgBlur), nil
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

	if err := bgClearFiles(dir, prefix); err != nil {
		return err
	}

	dstName := prefix + ext
	if err := bgCopyFile(imagePath, filepath.Join(dir, dstName)); err != nil {
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

// ---------- Per-platform background ----------

func (p *PlatformService) GetPlatformBackground(platformKey string) (AppBackgroundInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return AppBackgroundInfo{Opacity: defaultBgOpacity, Blur: defaultBgBlur}, nil
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
		return AppBackgroundInfo{Opacity: defaultBgOpacity, Blur: defaultBgBlur}, nil
	}
	return buildAppBgInfo(ps.Image, ps.Opacity, ps.Blur), nil
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

	if err := bgClearFiles(dir, prefix); err != nil {
		return err
	}

	dstName := prefix + ext
	if err := bgCopyFile(imagePath, filepath.Join(dir, dstName)); err != nil {
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
