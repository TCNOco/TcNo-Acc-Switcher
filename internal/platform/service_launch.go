package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/exeicon"
)

func (p *PlatformService) LaunchPlatform(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	p.mu.RLock()
	steamLauncher := launchSteamExe
	basicLauncher := launchBasicPlatform
	p.mu.RUnlock()
	if strings.EqualFold(platformKey, "Steam") {
		if steamLauncher == nil {
			return errors.New("steam launcher not configured")
		}
		return steamLauncher()
	}
	if basicLauncher == nil {
		return errors.New("basic launcher not configured")
	}
	return basicLauncher(platformKey)
}

func (p *PlatformService) LaunchPlatformAs(platformKey string, admin bool) error {
	platformKey = strings.TrimSpace(platformKey)
	p.mu.RLock()
	steamLauncherAs := launchSteamExeAs
	steamLauncher := launchSteamExe
	basicLauncherAs := launchBasicPlatformAs
	basicLauncher := launchBasicPlatform
	p.mu.RUnlock()
	if strings.EqualFold(platformKey, "Steam") {
		if steamLauncherAs != nil {
			return steamLauncherAs(admin)
		}
		if steamLauncher == nil {
			return errors.New("steam launcher not configured")
		}
		return steamLauncher()
	}
	if basicLauncherAs != nil {
		return basicLauncherAs(platformKey, admin)
	}
	if basicLauncher == nil {
		return errors.New("basic launcher not configured")
	}
	return basicLauncher(platformKey)
}

func (p *PlatformService) GetPlatformInstallFolder(platformKey string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.getPlatformInstallFolderUnlocked(platformKey)
}

func (p *PlatformService) OpenPlatformFolder(platformKey string) error {
	p.mu.RLock()
	folder, err := p.getPlatformInstallFolderUnlocked(platformKey)
	p.mu.RUnlock()
	if err != nil {
		return err
	}
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return fmt.Errorf("install location unknown for %s", strings.TrimSpace(platformKey))
	}
	st, err := os.Stat(folder)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", folder)
	}
	return OpenPathInFileManager(folder)
}

func (p *PlatformService) getPlatformInstallFolderUnlocked(platformKey string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return "", errors.New("empty platform")
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return "", err
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		return "", err
	}
	entry, err := parsePlatformEntry(raw, platformKey)
	if err != nil {
		return "", err
	}
	exeName := primaryExeName(entry)

	if strings.EqualFold(platformKey, "Steam") && resolveSteamExePath != nil {
		if ex, ok := resolveSteamExePath(); ok {
			return filepath.Dir(ex), nil
		}
	}

	if saved := strings.TrimSpace(settings.PlatformExePaths[platformKey]); saved != "" {
		if st, err := os.Stat(saved); err == nil && !st.IsDir() {
			return filepath.Dir(saved), nil
		}
	}

	defExisting := entry.ExeLocationDefault.FirstExistingExe()
	if defExisting != "" {
		return filepath.Dir(defExisting), nil
	}

	if found, ok := findExeViaStartMenuShortcuts(entry, exeName); ok {
		return filepath.Dir(found), nil
	}

	if defExpanded := entry.ExeLocationDefault.FirstExpanded(); defExpanded != "" {
		d := filepath.Dir(defExpanded)
		if d != "." && !strings.HasSuffix(d, ":") {
			return d, nil
		}
	}
	return "", nil
}

func (p *PlatformService) ResolvePlatformExeFullPath(platformKey string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.resolvePlatformExeFullPathUnlocked(platformKey)
}

func (p *PlatformService) resolvePlatformExeFullPathUnlocked(platformKey string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return "", errors.New("empty platform")
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return "", err
	}
	if saved := strings.TrimSpace(settings.PlatformExePaths[platformKey]); saved != "" {
		if st, err := os.Stat(saved); err == nil && !st.IsDir() {
			return filepath.Clean(saved), nil
		}
	}
	if strings.EqualFold(platformKey, "Steam") && resolveSteamExePath != nil {
		if ex, ok := resolveSteamExePath(); ok {
			return filepath.Clean(ex), nil
		}
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		return "", err
	}
	entry, err := parsePlatformEntry(raw, platformKey)
	if err != nil {
		return "", err
	}
	exeName := primaryExeName(entry)
	if exeName == "" {
		return "", errors.New("could not determine executable name")
	}
	folder, err := p.getPlatformInstallFolderUnlocked(platformKey)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(folder) == "" {
		return "", errors.New("install folder unknown")
	}
	return filepath.Join(folder, exeName), nil
}

func (p *PlatformService) GetPlatformExeIcon(platformKey string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	exe, err := p.resolvePlatformExeFullPathUnlocked(platformKey)
	if err != nil || exe == "" {
		return "", nil
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", nil
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err == nil {
		entry, err := parsePlatformEntry(raw, platformKey)
		if err == nil {
			d, err := ParseDescriptor(raw, platformKey)
			if err == nil && d.Extras.SearchStartMenuForIcon {
				if shortcutPath, ok := findStartMenuIconShortcut(entry); ok {
					www, err := WwwrootDir()
					if err == nil {
						if u, err := exeicon.EnsureShortcutCached(platformKey, filepath.Base(exe), shortcutPath, www); err == nil {
							return u, nil
						}
					}
				}
			}
		}
	}
	www, err := WwwrootDir()
	if err != nil {
		return "", nil
	}
	u, err := exeicon.EnsureCached(platformKey, exe, www)
	if err != nil {
		return "", nil
	}
	return u, nil
}

func (p *PlatformService) HasShortcutMainExe(platformKey string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	platformKey = strings.TrimSpace(platformKey)
	if strings.EqualFold(platformKey, "Steam") {
		return true, nil
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		return false, err
	}
	d, err := ParseDescriptor(raw, platformKey)
	if err != nil {
		return false, err
	}
	if d.Extras.ShortcutIncludeMainExe != nil && *d.Extras.ShortcutIncludeMainExe {
		return true, nil
	}
	return false, nil
}

func (p *PlatformService) seedDisabledPlatformsForFirstLaunch(settings *AppSettings, raw []byte, names []string) {
	if settings == nil {
		return
	}
	disabled := make(map[string]struct{}, len(names))
	foundCount := 0
	for _, platformName := range names {
		if p.platformDetected(settings, raw, platformName) {
			foundCount++
			continue
		}
		disabled[platformName] = struct{}{}
	}
	if foundCount == 0 {
		disabled = make(map[string]struct{}, len(names))
		for _, platformName := range names {
			if strings.EqualFold(platformName, "Steam") {
				continue
			}
			disabled[platformName] = struct{}{}
		}
	}
	settings.DisabledPlatforms = setToSortedSlice(disabled)
}

func (p *PlatformService) platformDetected(settings *AppSettings, raw []byte, platformName string) bool {
	if settings == nil {
		return false
	}
	if saved := strings.TrimSpace(settings.PlatformExePaths[platformName]); saved != "" {
		if st, err := os.Stat(saved); err == nil && !st.IsDir() {
			return true
		}
	}
	if strings.EqualFold(platformName, "Steam") && resolveSteamExePath != nil {
		if _, ok := resolveSteamExePath(); ok {
			return true
		}
	}
	entry, err := parsePlatformEntry(raw, platformName)
	if err != nil {
		return false
	}
	if entry.ExeLocationDefault.FirstExistingExe() != "" {
		return true
	}
	exeName := primaryExeName(entry)
	if exeName == "" {
		return false
	}
	_, ok := findExeViaStartMenuShortcuts(entry, exeName)
	return ok
}
