package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePlatformLaunchResult is returned before navigating to a platform page.
type ResolvePlatformLaunchResult struct {
	Ok                bool   `json:"ok"`
	NeedsManualLocate bool   `json:"needsManualLocate"`
	FoundViaShortcut  bool   `json:"foundViaShortcut"`
	SoughtExeName     string `json:"soughtExeName"`
	InitialPath       string `json:"initialPath"`
}

// ResolvePlatformLaunch checks saved path, default exe path, then OS-specific shortcut discovery.
func (p *PlatformService) ResolvePlatformLaunch(platformKey string) (ResolvePlatformLaunchResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return ResolvePlatformLaunchResult{}, errors.New("empty platform")
	}

	exeDir, err := ResolveExeDir()
	if err != nil {
		return ResolvePlatformLaunchResult{}, err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return ResolvePlatformLaunchResult{}, err
	}
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
	if err != nil {
		return ResolvePlatformLaunchResult{}, err
	}
	entry, err := parsePlatformEntry(raw, platformKey)
	if err != nil {
		return ResolvePlatformLaunchResult{}, err
	}
	exeName := primaryExeName(entry)
	if exeName == "" {
		return ResolvePlatformLaunchResult{}, errors.New("could not determine executable name for platform")
	}

	defExpanded := expandWindowsPath(strings.TrimSpace(entry.ExeLocationDefault))

	if saved := strings.TrimSpace(settings.PlatformExePaths[platformKey]); saved != "" {
		if st, err := os.Stat(saved); err == nil && !st.IsDir() {
			return ResolvePlatformLaunchResult{Ok: true}, nil
		}
	}

	if defExpanded != "" {
		if st, err := os.Stat(defExpanded); err == nil && !st.IsDir() {
			return ResolvePlatformLaunchResult{Ok: true}, nil
		}
	}

	if found, ok := findExeViaStartMenuShortcuts(entry, exeName); ok {
		if settings.PlatformExePaths == nil {
			settings.PlatformExePaths = map[string]string{}
		}
		settings.PlatformExePaths[platformKey] = found
		if err := saveSettingsAtomic(exeDir, settings); err != nil {
			return ResolvePlatformLaunchResult{}, err
		}
		return ResolvePlatformLaunchResult{Ok: true, FoundViaShortcut: true}, nil
	}

	initial := filepath.Dir(defExpanded)
	if initial == "." || strings.HasSuffix(initial, ":") {
		initial = ""
	}
	return ResolvePlatformLaunchResult{
		NeedsManualLocate: true,
		SoughtExeName:     exeName,
		InitialPath:       initial,
	}, nil
}

// ConfirmPlatformExePath validates basename and saves the full path to settings.
func (p *PlatformService) ConfirmPlatformExePath(platformKey, exeFullPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("empty platform")
	}

	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
	if err != nil {
		return err
	}
	entry, err := parsePlatformEntry(raw, platformKey)
	if err != nil {
		return err
	}
	wantName := primaryExeName(entry)
	if wantName == "" {
		return errors.New("could not determine executable name for platform")
	}

	exeFullPath = strings.TrimSpace(exeFullPath)
	if exeFullPath == "" {
		return errors.New("empty path")
	}
	st, err := os.Stat(exeFullPath)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return errors.New("path is a directory")
	}
	if !strings.EqualFold(filepath.Base(exeFullPath), wantName) {
		return fmt.Errorf("expected %s, got %s", wantName, filepath.Base(exeFullPath))
	}
	if settings.PlatformExePaths == nil {
		settings.PlatformExePaths = map[string]string{}
	}
	settings.PlatformExePaths[platformKey] = filepath.Clean(exeFullPath)
	return saveSettingsAtomic(exeDir, settings)
}
