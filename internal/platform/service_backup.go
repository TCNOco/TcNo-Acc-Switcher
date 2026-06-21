package platform

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (p *PlatformService) PickPlatformsJSON() (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("application not initialised")
	}
	sel, err := app.Dialog.OpenFile().
		SetTitle("Locate Platforms.json").
		AddFilter("JSON", "*.json").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sel), nil
}

func (p *PlatformService) PickProfileImageFile() (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("application not initialised")
	}
	sel, err := app.Dialog.OpenFile().
		SetTitle("Choose profile image").
		AddFilter("Images", "*.png;*.jpg;*.jpeg;*.webp;*.gif").
		AddFilter("Video avatars", "*.webm;*.mp4").
		AddFilter("All supported", "*.png;*.jpg;*.jpeg;*.webp;*.gif;*.webm;*.mp4").
		AddFilter("All files", "*.*").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sel), nil
}

func (p *PlatformService) ApplyPlatformsJSONFile(sourcePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return errors.New("empty path")
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	if _, err := parsePlatformNames(data); err != nil {
		return err
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	ud := UserDataDir(exeDir)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(ud, "Platforms.json")
	if err := atomicWriteBytes(dest, data, 0o644); err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	settings.PlatformsJSONPath = ""
	settings.PlatformOrder = nil
	invalidatePlatformsJSONCache()
	return saveSettingsAtomic(exeDir, settings)
}

func (p *PlatformService) RestoreDefaultPlatformsJSON() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(embeddedPlatformsJSON) == 0 {
		return errors.New("embedded platforms data missing")
	}
	names, err := parsePlatformNames(embeddedPlatformsJSON)
	if err != nil {
		return err
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	ud := UserDataDir(exeDir)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(ud, "Platforms.json")
	if err := atomicWriteBytes(dest, bytes.Clone(embeddedPlatformsJSON), 0o644); err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	settings.PlatformsJSONPath = ""
	p.seedDisabledPlatformsForFirstLaunch(&settings, embeddedPlatformsJSON, names)
	invalidatePlatformsJSONCache()
	return saveSettingsAtomic(exeDir, settings)
}
