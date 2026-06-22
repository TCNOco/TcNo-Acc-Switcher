package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/stats"
)

type platformsFile struct {
	Platforms map[string]json.RawMessage `json:"Platforms"`
	Version   string                     `json:"Version,omitempty"`
}

type PlatformStartup struct {
	HomePlatformOrder     []string                        `json:"homePlatformOrder"`
	AllPlatformNames      []string                        `json:"allPlatformNames"`
	DisabledPlatformNames []string                        `json:"disabledPlatformNames"`
	PlatformsFileMissing  bool                            `json:"platformsFileMissing"`
	PlatformAccountCounts map[string]int                  `json:"platformAccountCounts"`
	PlatformTagCounts     map[string]PlatformTagCountInfo `json:"platformTagCounts"`
	Language              string                          `json:"language"`
	Theme                 string                          `json:"theme,omitempty"`
	CliNavigateHint       string                          `json:"cliNavigateHint,omitempty"`

	OfflineMode           bool   `json:"offlineMode"`
	ProtocolEnabled       bool   `json:"protocolEnabled"`
	ExitToTray            bool   `json:"exitToTray"`
	DiscordRpc            bool   `json:"discordRpc"`
	DiscordRpcShare       bool   `json:"discordRpcShare"`
	MinimizeOnSwitch      bool   `json:"minimizeOnSwitch"`
	StartTrayWithWindows  bool   `json:"startTrayWithWindows"`
	StartProgramCentered  bool   `json:"startProgramCentered"`
	AnimationsEnabled     bool   `json:"animationsEnabled"`
	StatsEnabled          bool   `json:"statsEnabled"`
	StatsShare            bool   `json:"statsShare"`
	CrashReportAutoSubmit bool   `json:"crashReportAutoSubmit"`
	ThemeAccentPreset     string `json:"themeAccentPreset"`
	ThemeAccentCustom     string `json:"themeAccentCustom"`
	AppVersion            string `json:"appVersion"`
}

// PlatformTagCountInfo is a per-platform tag statistic (used in startup skeleton hints).
type PlatformTagCountInfo struct {
	TagCount           int `json:"tagCount"`
	TaggedAccountCount int `json:"taggedAccountCount"`
}

type PlatformService struct {
	mu sync.RWMutex
}

func (p *PlatformService) withSettingsRead(fn func(s *AppSettings) error) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	exeDir, err := ResolveExeDir()
	if err != nil {
		return err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return err
	}
	return fn(&s)
}

func (p *PlatformService) withSettingsWrite(fn func(s *AppSettings) error) error {
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
	if err := fn(&s); err != nil {
		return err
	}
	return saveSettingsAtomic(exeDir, s)
}

func (p *PlatformService) GetStartup() (PlatformStartup, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	exeDir, err := ResolveExeDir()
	if err != nil {
		return PlatformStartup{}, err
	}
	settingsMissing := !settingsFileExists(exeDir)
	settings, err := loadSettings(exeDir)
	if err != nil {
		return PlatformStartup{}, err
	}
	raw, err := LoadPlatformsJSON(exeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return PlatformStartup{
				Language:              settings.Language,
				Theme:                 sanitizeThemeID(settings.Theme),
				PlatformsFileMissing:  true,
				PlatformAccountCounts: map[string]int{},
				PlatformTagCounts:     map[string]PlatformTagCountInfo{},
				CliNavigateHint:       ConsumeStartupNavigateHint(),
				OfflineMode:           settings.OfflineMode,
				ProtocolEnabled:       settings.ProtocolEnabled,
				ExitToTray:            settings.ExitToTray,
				DiscordRpc:            settings.DiscordRpc,
				DiscordRpcShare:       settings.DiscordRpcShare,
				MinimizeOnSwitch:      settings.MinimizeOnSwitch,
				StartTrayWithWindows:  settings.StartTrayWithWindows,
				StartProgramCentered:  settings.StartProgramCentered,
				AnimationsEnabled:     settings.AnimationsEnabled,
				StatsEnabled:          settings.StatsEnabled,
				StatsShare:            settings.StatsShare,
				CrashReportAutoSubmit: settings.CrashReportAutoSubmit,
				ThemeAccentPreset:     settings.ThemeAccentPreset,
				ThemeAccentCustom:     settings.ThemeAccentCustom,
				AppVersion:            appVersionFromBuildConfig(),
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
	accountCounts := resolveStartupAccountCounts(names, settings.StatsEnabled)
	tagCounts := resolveStartupTagCounts(names, settings.StatsEnabled)
	return PlatformStartup{
		HomePlatformOrder:     home,
		AllPlatformNames:      names,
		DisabledPlatformNames: disList,
		PlatformsFileMissing:  false,
		PlatformAccountCounts: accountCounts,
		PlatformTagCounts:     tagCounts,
		Language:              settings.Language,
		Theme:                 sanitizeThemeID(settings.Theme),
		CliNavigateHint:       nav,
		OfflineMode:           settings.OfflineMode,
		ProtocolEnabled:       settings.ProtocolEnabled,
		ExitToTray:            settings.ExitToTray,
		DiscordRpc:            settings.DiscordRpc,
		DiscordRpcShare:       settings.DiscordRpcShare,
		MinimizeOnSwitch:      settings.MinimizeOnSwitch,
		StartTrayWithWindows:  settings.StartTrayWithWindows,
		StartProgramCentered:  settings.StartProgramCentered,
		AnimationsEnabled:     settings.AnimationsEnabled,
		StatsEnabled:          settings.StatsEnabled,
		StatsShare:            settings.StatsShare,
		CrashReportAutoSubmit: settings.CrashReportAutoSubmit,
		ThemeAccentPreset:     settings.ThemeAccentPreset,
		ThemeAccentCustom:     settings.ThemeAccentCustom,
		AppVersion:            appVersionFromBuildConfig(),
	}, nil
}

func (p *PlatformService) ReadSettings() (PlatformStartup, error) {
	return p.GetStartup()
}

type SettingsBatchUpdate struct {
	OfflineMode           *bool   `json:"offlineMode,omitempty"`
	ProtocolEnabled       *bool   `json:"protocolEnabled,omitempty"`
	ExitToTray            *bool   `json:"exitToTray,omitempty"`
	DiscordRpc            *bool   `json:"discordRpc,omitempty"`
	DiscordRpcShare       *bool   `json:"discordRpcShare,omitempty"`
	MinimizeOnSwitch      *bool   `json:"minimizeOnSwitch,omitempty"`
	StartTrayWithWindows  *bool   `json:"startTrayWithWindows,omitempty"`
	StartProgramCentered  *bool   `json:"startProgramCentered,omitempty"`
	AnimationsEnabled     *bool   `json:"animationsEnabled,omitempty"`
	StatsEnabled          *bool   `json:"statsEnabled,omitempty"`
	StatsShare            *bool   `json:"statsShare,omitempty"`
	CrashReportAutoSubmit *bool   `json:"crashReportAutoSubmit,omitempty"`
	Language              *string `json:"language,omitempty"`
	Theme                 *string `json:"theme,omitempty"`
	ThemeAccentPreset     *string `json:"themeAccentPreset,omitempty"`
	ThemeAccentCustom     *string `json:"themeAccentCustom,omitempty"`
}

func (p *PlatformService) UpdateSettings(req SettingsBatchUpdate) error {
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
	effects := applySettingsBatchUpdate(&s, req)
	if !effects.dirty {
		return nil
	}
	if err := saveSettingsAtomic(exeDir, s); err != nil {
		return err
	}
	if effects.statsEnabled != nil {
		stats.SetStatsCollectionEnabled(*effects.statsEnabled)
	}
	if effects.offlineMode != nil {
		appclient.SetOfflineMode(*effects.offlineMode)
	}
	if effects.discordPresenceRefresh {
		TriggerDiscordPresenceRefresh()
	}
	return nil
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
	type item struct {
		orig  string
		lower string
	}
	items := make([]item, len(s))
	for i, v := range s {
		items[i] = item{orig: v, lower: strings.ToLower(v)}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].lower < items[j].lower
	})
	for i, v := range items {
		s[i] = v.orig
	}
}
