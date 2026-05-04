package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Launch hooks are set from main to avoid an import cycle with internal/steam.
var (
	saveSteamFolderFromExe func(exeFullPath string) error
	resolveSteamExePath    func() (exePath string, ok bool)
	resetSteamSettings     func() error
	launchSteamExe          func() error
	launchBasicPlatform     func(platformKey string) error
	launchSteamExeAs        func(forceAdmin bool) error
	launchBasicPlatformAs   func(platformKey string, forceAdmin bool) error
)

func SetSteamLaunchHooks(saveExe func(exeFullPath string) error, resolve func() (exePath string, ok bool)) {
	saveSteamFolderFromExe = saveExe
	resolveSteamExePath = resolve
}

func SetSteamReset(fn func() error) {
	resetSteamSettings = fn
}

func SetPlatformLaunchers(steam func() error, basic func(platformKey string) error) {
	launchSteamExe = steam
	launchBasicPlatform = basic
}

func SetPlatformLaunchAs(steamAs func(forceAdmin bool) error, basicAs func(platformKey string, forceAdmin bool) error) {
	launchSteamExeAs = steamAs
	launchBasicPlatformAs = basicAs
}

type ResolvePlatformLaunchResult struct {
	Ok                bool   `json:"ok"`
	NeedsManualLocate bool   `json:"needsManualLocate"`
	FoundViaShortcut  bool   `json:"foundViaShortcut"`
	SoughtExeName     string `json:"soughtExeName"`
	InitialPath       string `json:"initialPath"`
}

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

	defExisting := entry.ExeLocationDefault.FirstExistingExe()
	defExpanded := entry.ExeLocationDefault.FirstExpanded()

	if strings.EqualFold(platformKey, "Steam") && resolveSteamExePath != nil {
		if p, ok := resolveSteamExePath(); ok {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return ResolvePlatformLaunchResult{
					Ok:            true,
					SoughtExeName: exeName,
					InitialPath:   filepath.Dir(p),
				}, nil
			}
		}
	}

	if saved := strings.TrimSpace(settings.PlatformExePaths[platformKey]); saved != "" {
		if st, err := os.Stat(saved); err == nil && !st.IsDir() {
			return ResolvePlatformLaunchResult{
				Ok:            true,
				SoughtExeName: exeName,
				InitialPath:   filepath.Dir(saved),
			}, nil
		}
	}

	if defExisting != "" {
		return ResolvePlatformLaunchResult{
			Ok:            true,
			SoughtExeName: exeName,
			InitialPath:   filepath.Dir(defExisting),
		}, nil
	}

	if found, ok := findExeViaStartMenuShortcuts(entry, exeName); ok {
		if strings.EqualFold(platformKey, "Steam") && saveSteamFolderFromExe != nil {
			if err := saveSteamFolderFromExe(found); err != nil {
				return ResolvePlatformLaunchResult{}, err
			}
			return ResolvePlatformLaunchResult{
				Ok:                true,
				FoundViaShortcut:  true,
				SoughtExeName:     exeName,
				InitialPath:       filepath.Dir(found),
			}, nil
		}
		if settings.PlatformExePaths == nil {
			settings.PlatformExePaths = map[string]string{}
		}
		settings.PlatformExePaths[platformKey] = found
		if err := saveSettingsAtomic(exeDir, settings); err != nil {
			return ResolvePlatformLaunchResult{}, err
		}
		return ResolvePlatformLaunchResult{
			Ok:               true,
			FoundViaShortcut: true,
			SoughtExeName:    exeName,
			InitialPath:      filepath.Dir(found),
		}, nil
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
	if strings.EqualFold(platformKey, "Steam") && saveSteamFolderFromExe != nil {
		return saveSteamFolderFromExe(exeFullPath)
	}
	if settings.PlatformExePaths == nil {
		settings.PlatformExePaths = map[string]string{}
	}
	settings.PlatformExePaths[platformKey] = filepath.Clean(exeFullPath)
	return saveSettingsAtomic(exeDir, settings)
}
