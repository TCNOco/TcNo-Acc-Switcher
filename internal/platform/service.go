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

	"TcNo-Acc-Switcher/internal/exeicon"
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
	settings, err := loadSettings(exeDir)
	if err != nil {
		return PlatformStartup{}, err
	}
	path := p.resolvePlatformsPath(exeDir, settings)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PlatformStartup{
				Language:             settings.Language,
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
		CliNavigateHint:       nav,
	}, nil
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
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
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
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
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
	sel, err := app.Dialog.OpenFileWithOptions(nil).
		SetTitle("Locate Platforms.json").
		AddFilter("JSON", "*.json").
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
	dest := filepath.Join(exeDir, "Platforms.json")
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
	if _, err := parsePlatformNames(embeddedPlatformsJSON); err != nil {
		return err
	}
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	dest := filepath.Join(exeDir, "Platforms.json")
	if err := atomicWriteBytes(dest, bytes.Clone(embeddedPlatformsJSON), 0o644); err != nil {
		return err
	}
	settings, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	settings.PlatformsJSONPath = ""
	return saveSettingsAtomic(exeDir, settings)
}

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
	return filepath.Join(exeDir, "Platforms.json")
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
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
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

	defExpanded := ExpandWindowsPath(strings.TrimSpace(entry.ExeLocationDefault))
	if defExpanded != "" {
		if st, err := os.Stat(defExpanded); err == nil && !st.IsDir() {
			return filepath.Dir(defExpanded), nil
		}
	}

	if found, ok := findExeViaStartMenuShortcuts(entry, exeName); ok {
		return filepath.Dir(found), nil
	}

	if defExpanded != "" {
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
	raw, err := os.ReadFile(p.resolvePlatformsPath(exeDir, settings))
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
	p.mu.Lock()
	defer p.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	if strings.EqualFold(platformKey, "Steam") {
		if launchSteamExe == nil {
			return errors.New("steam launcher not configured")
		}
		return launchSteamExe()
	}
	if launchBasicPlatform == nil {
		return errors.New("basic launcher not configured")
	}
	return launchBasicPlatform(platformKey)
}

func (p *PlatformService) LaunchPlatformAs(platformKey string, admin bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	if strings.EqualFold(platformKey, "Steam") {
		if launchSteamExeAs != nil {
			return launchSteamExeAs(admin)
		}
		if launchSteamExe == nil {
			return errors.New("steam launcher not configured")
		}
		return launchSteamExe()
	}
	if launchBasicPlatformAs != nil {
		return launchBasicPlatformAs(platformKey, admin)
	}
	if launchBasicPlatform == nil {
		return errors.New("basic launcher not configured")
	}
	return launchBasicPlatform(platformKey)
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
	settings, err := loadSettings(exeDir)
	if err != nil {
		return false, err
	}
	raw, err := os.ReadFile(resolvePlatformsPath(exeDir, settings))
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
