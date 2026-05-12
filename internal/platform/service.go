package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/exeicon"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type platformsFile struct {
	Platforms map[string]json.RawMessage `json:"Platforms"`
}

type PlatformStartup struct {
	HomePlatformOrder     []string `json:"homePlatformOrder"`
	AllPlatformNames      []string `json:"allPlatformNames"`
	DisabledPlatformNames []string `json:"disabledPlatformNames"`
	PlatformsFileMissing  bool     `json:"platformsFileMissing"`
	Language              string   `json:"language"`
	Theme                 string   `json:"theme,omitempty"`
	// One-shot SPA route from CLI (e.g. open Steam page after elevated restart).
	CliNavigateHint string `json:"cliNavigateHint,omitempty"`
}

type PlatformService struct {
	mu sync.Mutex
}

func (p *PlatformService) GetStartup() (PlatformStartup, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	exeDir, err := ResolveExeDir()
	if err != nil {
		return PlatformStartup{}, err
	}
	_, settingsStatErr := os.Stat(settingsPath(exeDir))
	settingsMissing := settingsStatErr != nil && os.IsNotExist(settingsStatErr)
	settings, err := loadSettings(exeDir)
	if err != nil {
		return PlatformStartup{}, err
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return PlatformStartup{
				Language:             settings.Language,
				Theme:                sanitizeThemeID(settings.Theme),
				PlatformsFileMissing: true,
				CliNavigateHint:      ConsumeStartupNavigateHint(),
			}, nil
		}
		return PlatformStartup{}, err
	}
	names, err := parsePlatformNames(raw)
	if err != nil {
		return PlatformStartup{}, err
	}
	if settingsMissing {
		p.seedDisabledPlatformsForFirstLaunch(&settings, raw, names)
		if err := saveSettingsAtomic(exeDir, settings); err != nil {
			return PlatformStartup{}, err
		}
	}
	disabled := sliceToSet(settings.DisabledPlatforms)
	home := computeHomeOrder(names, disabled, settings.PlatformOrder)
	disList := make([]string, 0, len(disabled))
	for _, n := range names {
		if _, ok := disabled[n]; ok {
			disList = append(disList, n)
		}
	}
	sortStringsFold(disList)
	nav := ConsumeStartupNavigateHint()
	return PlatformStartup{
		HomePlatformOrder:     home,
		AllPlatformNames:      names,
		DisabledPlatformNames: disList,
		PlatformsFileMissing:  false,
		Language:              settings.Language,
		Theme:                 sanitizeThemeID(settings.Theme),
		CliNavigateHint:       nav,
	}, nil
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

func (p *PlatformService) GetLanguage() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return "", err
	}
	if settings.Language == "" {
		return "en-US", nil
	}
	return settings.Language, nil
}

func (p *PlatformService) SetLanguage(code string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	settings.Language = strings.TrimSpace(code)
	if settings.Language == "" {
		settings.Language = "en-US"
	}
	return saveSettingsAtomic(exeDir, settings)
}

func sanitizeThemeID(id string) string {
	s := strings.TrimSpace(id)
	if s == "" || strings.EqualFold(s, "default") {
		return ""
	}
	if len(s) > 64 {
		return ""
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			continue
		default:
			return ""
		}
	}
	return s
}

func sanitizeThemeAccentPreset(id string) string {
	s := strings.TrimSpace(id)
	if s == "" {
		return ""
	}
	if strings.EqualFold(s, "custom") {
		return "custom"
	}
	if strings.EqualFold(s, "windows") {
		return "windows"
	}
	if len(s) > 64 {
		return ""
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			continue
		default:
			return ""
		}
	}
	return s
}

func sanitizeHexColor(value string) string {
	s := strings.TrimSpace(value)
	if len(s) != 7 || s[0] != '#' {
		return ""
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
			continue
		default:
			return ""
		}
	}
	return strings.ToLower(s)
}

func (p *PlatformService) GetTheme() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return "", err
	}
	return sanitizeThemeID(settings.Theme), nil
}

func (p *PlatformService) GetThemeAccentPreset() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return "", err
	}
	return sanitizeThemeAccentPreset(settings.ThemeAccentPreset), nil
}

func (p *PlatformService) GetThemeAccentCustom() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return "", err
	}
	return sanitizeHexColor(settings.ThemeAccentCustom), nil
}

func (p *PlatformService) GetWindowsAccentColor() string {
	return CurrentWindowsAccentColor()
}

func (p *PlatformService) GetAppVersion() string {
	return appVersionFromBuildConfig()
}

func (p *PlatformService) SetTheme(themeID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	nextTheme := sanitizeThemeID(themeID)
	if settings.Theme != nextTheme {
		settings.ThemeAccentPreset = ""
		settings.ThemeAccentCustom = ""
	}
	settings.Theme = nextTheme
	return saveSettingsAtomic(exeDir, settings)
}

func (p *PlatformService) SetThemeAccentPreset(preset string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	settings.ThemeAccentPreset = sanitizeThemeAccentPreset(preset)
	return saveSettingsAtomic(exeDir, settings)
}

func (p *PlatformService) SetThemeAccentCustom(color string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	settings.ThemeAccentCustom = sanitizeHexColor(color)
	return saveSettingsAtomic(exeDir, settings)
}

func (p *PlatformService) SaveHomeOrder(order []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	allNames, err := parsePlatformNames(raw)
	if err != nil {
		return err
	}
	disabled := sliceToSet(settings.DisabledPlatforms)
	enabled := make(map[string]struct{})
	for _, n := range allNames {
		if _, hid := disabled[n]; !hid {
			enabled[n] = struct{}{}
		}
	}
	if len(order) != len(enabled) {
		return errors.New("order length does not match enabled platforms")
	}
	seen := make(map[string]struct{})
	for _, n := range order {
		if _, ok := enabled[n]; !ok {
			return errors.New("invalid platform in order: " + n)
		}
		if _, dup := seen[n]; dup {
			return errors.New("duplicate platform in order")
		}
		seen[n] = struct{}{}
	}
	settings.PlatformOrder = append([]string(nil), order...)
	return saveSettingsAtomic(exeDir, settings)
}

func (p *PlatformService) SetDisabledPlatforms(disabled []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	allNames, err := parsePlatformNames(raw)
	if err != nil {
		return err
	}
	valid := make(map[string]struct{}, len(allNames))
	for _, n := range allNames {
		valid[n] = struct{}{}
	}
	nextDis := make(map[string]struct{})
	for _, n := range disabled {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := valid[n]; !ok {
			continue
		}
		nextDis[n] = struct{}{}
	}
	prevDis := sliceToSet(settings.DisabledPlatforms)

	var order []string
	seen := make(map[string]struct{})
	for _, n := range settings.PlatformOrder {
		if _, d := nextDis[n]; d {
			continue
		}
		if _, ok := valid[n]; !ok {
			continue
		}
		order = append(order, n)
		seen[n] = struct{}{}
	}
	var newlyEnabled []string
	for _, n := range allNames {
		_, was := prevDis[n]
		_, now := nextDis[n]
		if was && !now {
			if _, ok := seen[n]; !ok {
				newlyEnabled = append(newlyEnabled, n)
			}
		}
	}
	sortStringsFold(newlyEnabled)
	for _, n := range newlyEnabled {
		order = append(order, n)
		seen[n] = struct{}{}
	}
	for _, n := range allNames {
		if _, d := nextDis[n]; d {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		order = append(order, n)
	}
	settings.DisabledPlatforms = setToSortedSlice(nextDis)
	settings.PlatformOrder = order
	return saveSettingsAtomic(exeDir, settings)
}

func (p *PlatformService) SetPlatformExePath(platformKey, exePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	if settings.PlatformExePaths == nil {
		settings.PlatformExePaths = map[string]string{}
	}
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		delete(settings.PlatformExePaths, platformKey)
	} else {
		settings.PlatformExePaths[platformKey] = exePath
	}
	return saveSettingsAtomic(exeDir, settings)
}

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

// PickProfileImageFile opens a native file picker for a single image or short video avatar file.
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
	return saveSettingsAtomic(exeDir, settings)
}

// ResolvePlatformsJSONPath returns the path to the base Platforms.json file
// (not merged with Platforms.custom.json). Use [LoadPlatformsJSON] to read the
// effective configuration.
func ResolvePlatformsJSONPath(exeDir string) (string, error) {
	s, err := loadSettings(exeDir)
	if err != nil {
		return "", err
	}
	return resolvePlatformsPath(exeDir, s), nil
}

func (p *PlatformService) resolvePlatformsPath(exeDir string, s AppSettings) string {
	return resolvePlatformsPath(exeDir, s)
}

func resolvePlatformsPath(exeDir string, s AppSettings) string {
	if rel := strings.TrimSpace(s.PlatformsJSONPath); rel != "" {
		if filepath.IsAbs(rel) {
			return filepath.Clean(rel)
		}
		return filepath.Clean(filepath.Join(exeDir, rel))
	}
	return filepath.Join(UserDataDir(exeDir), "Platforms.json")
}

func parsePlatformNames(raw []byte) ([]string, error) {
	var pf platformsFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		return nil, err
	}
	if pf.Platforms == nil {
		return nil, errors.New("invalid Platforms.json: missing Platforms")
	}
	out := make([]string, 0, len(pf.Platforms))
	for k := range pf.Platforms {
		out = append(out, k)
	}
	sortStringsFold(out)
	return out, nil
}

func sliceToSet(s []string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, x := range s {
		x = strings.TrimSpace(x)
		if x != "" {
			m[x] = struct{}{}
		}
	}
	return m
}

func setToSortedSlice(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStringsFold(out)
	return out
}

func computeHomeOrder(all []string, disabled map[string]struct{}, savedOrder []string) []string {
	enabled := make([]string, 0, len(all))
	for _, n := range all {
		if _, d := disabled[n]; !d {
			enabled = append(enabled, n)
		}
	}
	enSet := make(map[string]struct{}, len(enabled))
	for _, n := range enabled {
		enSet[n] = struct{}{}
	}
	var out []string
	seen := make(map[string]struct{})
	for _, n := range savedOrder {
		if _, ok := enSet[n]; !ok {
			continue
		}
		if _, d := disabled[n]; d {
			continue
		}
		out = append(out, n)
		seen[n] = struct{}{}
	}
	var rest []string
	for _, n := range enabled {
		if _, ok := seen[n]; !ok {
			rest = append(rest, n)
		}
	}
	sortStringsFold(rest)
	return append(out, rest...)
}

func sortStringsFold(s []string) {
	sort.Slice(s, func(i, j int) bool {
		return strings.ToLower(s[i]) < strings.ToLower(s[j])
	})
}

func (p *PlatformService) GetPlatformSettings(platformKey string) (PlatformSettings, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return LoadPlatformSettings(platformKey)
}

func (p *PlatformService) SavePlatformSettings(platformKey string, s PlatformSettings) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return SavePlatformSettings(platformKey, s)
}

func (p *PlatformService) ResetPlatformSettings(platformKey string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return resetPlatformJSONToDefaults(platformKey)
}

func (p *PlatformService) GetPlatformInstallFolder(platformKey string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.getPlatformInstallFolderUnlocked(platformKey)
}

func (p *PlatformService) OpenPlatformFolder(platformKey string) error {
	p.mu.Lock()
	folder, err := p.getPlatformInstallFolderUnlocked(platformKey)
	p.mu.Unlock()
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

// getPlatformInstallFolderUnlocked: caller must hold p.mu.
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
	p.mu.Lock()
	defer p.mu.Unlock()
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
	p.mu.Lock()
	defer p.mu.Unlock()
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

func (p *PlatformService) LaunchPlatform(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	p.mu.Lock()
	steamLauncher := launchSteamExe
	basicLauncher := launchBasicPlatform
	p.mu.Unlock()
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
	p.mu.Lock()
	steamLauncherAs := launchSteamExeAs
	steamLauncher := launchSteamExe
	basicLauncherAs := launchBasicPlatformAs
	basicLauncher := launchBasicPlatform
	p.mu.Unlock()
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

func (p *PlatformService) HasShortcutMainExe(platformKey string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
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

func (p *PlatformService) GetProtocolEnabled() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.ProtocolEnabled, nil
}

func (p *PlatformService) SetProtocolEnabled(enabled bool) error {
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
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)

	if enabled {
		if err := winutil.RegisterProtocol(self); err != nil {
			return err
		}
	} else {
		_ = winutil.UnregisterProtocol()
	}

	s.ProtocolEnabled = enabled
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) GetOfflineMode() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.OfflineMode, nil
}

func (p *PlatformService) GetStatsEnabled() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.StatsEnabled, nil
}

func (p *PlatformService) SetStatsEnabled(enabled bool) error {
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
	s.StatsEnabled = enabled
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) GetStatsShare() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.StatsShare, nil
}

func (p *PlatformService) SetStatsShare(enabled bool) error {
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
	s.StatsShare = enabled
	return saveSettingsAtomic(exeDir, s)
}

// GetStatsReport returns local statistics for display in the settings modal.
func (p *PlatformService) GetStatsReport() (StatsReport, error) {
	p.mu.Lock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		p.mu.Unlock()
		return StatsReport{}, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		p.mu.Unlock()
		return StatsReport{}, err
	}
	share := s.StatsShare
	p.mu.Unlock()

	data, err := stats.GetReportData()
	if err != nil {
		return StatsReport{}, err
	}
	return assembleStatsReport(data, share), nil
}

// ResetStatistics clears collected statistics and assigns a new anonymous UUID.
func (p *PlatformService) ResetStatistics() error {
	return stats.ResetStatistics()
}

// StatsRecordPageVisit increments the visit counter for a SPA route path (e.g. "/", "/settings").
func (p *PlatformService) StatsRecordPageVisit(pagePath string) error {
	return stats.RecordPageVisit(pagePath)
}

// StatsAddPageTime adds dwell time in seconds for a SPA route path.
func (p *PlatformService) StatsAddPageTime(pagePath string, seconds int) error {
	return stats.AddPageTime(pagePath, seconds)
}

func (p *PlatformService) SetOfflineMode(enabled bool) error {
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
	s.OfflineMode = enabled
	if enabled {
		s.DiscordRpc = false
		s.DiscordRpcShare = false
	}
	if err := saveSettingsAtomic(exeDir, s); err != nil {
		return err
	}
	appclient.SetOfflineMode(enabled)
	TriggerDiscordPresenceRefresh()
	return nil
}

func (p *PlatformService) GetDiscordRpc() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.DiscordRpc, nil
}

func (p *PlatformService) SetDiscordRpc(enabled bool) error {
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
	if s.OfflineMode {
		enabled = false
	}
	s.DiscordRpc = enabled
	if !enabled {
		s.DiscordRpcShare = false
	}
	if err := saveSettingsAtomic(exeDir, s); err != nil {
		return err
	}
	TriggerDiscordPresenceRefresh()
	return nil
}

func (p *PlatformService) GetDiscordRpcShare() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.DiscordRpc && s.DiscordRpcShare, nil
}

func (p *PlatformService) SetDiscordRpcShare(enabled bool) error {
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
	if s.OfflineMode || !s.DiscordRpc {
		enabled = false
	}
	s.DiscordRpcShare = enabled
	if err := saveSettingsAtomic(exeDir, s); err != nil {
		return err
	}
	TriggerDiscordPresenceRefresh()
	return nil
}

func (p *PlatformService) GetExitToTray() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.ExitToTray, nil
}

func (p *PlatformService) SetExitToTray(enabled bool) error {
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
	s.ExitToTray = enabled
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) GetMinimizeOnSwitch() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.MinimizeOnSwitch, nil
}

func (p *PlatformService) SetMinimizeOnSwitch(enabled bool) error {
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
	s.MinimizeOnSwitch = enabled
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) GetStartTrayWithWindows() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.StartTrayWithWindows, nil
}

func (p *PlatformService) SetStartTrayWithWindows(enabled bool) error {
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
	prev := s.StartTrayWithWindows
	s.StartTrayWithWindows = enabled
	if err := saveSettingsAtomic(exeDir, s); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		s.StartTrayWithWindows = prev
		_ = saveSettingsAtomic(exeDir, s)
		return err
	}
	self = filepath.Clean(self)
	if err := winutil.SetRunAtStartupTray(self, enabled); err != nil {
		s.StartTrayWithWindows = prev
		_ = saveSettingsAtomic(exeDir, s)
		return err
	}
	return nil
}

func (p *PlatformService) GetStartProgramCentered() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return false, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	return s.StartProgramCentered, nil
}

func (p *PlatformService) SetStartProgramCentered(enabled bool) error {
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
	s.StartProgramCentered = enabled
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) GetDesktopHomeShortcutExists() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return winutil.HomeDesktopShortcutExists(), nil
}

func (p *PlatformService) SetDesktopHomeShortcut(create bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return winutil.SetHomeDesktopShortcut(create)
}
