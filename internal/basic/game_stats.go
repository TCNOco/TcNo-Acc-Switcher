package basic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/htmlquery"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

type gameStatsFile struct {
	StatsDefinitions        map[string]gameDefinition `json:"StatsDefinitions"`
	PlatformCompatibilities map[string][]string       `json:"PlatformCompatibilities"`
}

func mergeGameStatsCustom(cfg *gameStatsFile, customRaw []byte) error {
	if len(bytes.TrimSpace(customRaw)) == 0 {
		return nil
	}
	var c gameStatsFile
	if err := json.Unmarshal(customRaw, &c); err != nil {
		return fmt.Errorf("parse GameStats.custom.json: %w", err)
	}
	if c.StatsDefinitions != nil {
		if cfg.StatsDefinitions == nil {
			cfg.StatsDefinitions = map[string]gameDefinition{}
		}
		for k, v := range c.StatsDefinitions {
			cfg.StatsDefinitions[k] = v
		}
	}
	if c.PlatformCompatibilities != nil {
		if cfg.PlatformCompatibilities == nil {
			cfg.PlatformCompatibilities = map[string][]string{}
		}
		for k, v := range c.PlatformCompatibilities {
			cfg.PlatformCompatibilities[k] = v
		}
	}
	return nil
}

type gameDefinition struct {
	UniqueID       string                        `json:"UniqueId"`
	Indicator      string                        `json:"Indicator"`
	URL            string                        `json:"Url"`
	RequestCookies string                        `json:"RequestCookies"`
	Vars           map[string]gameStatVarDef     `json:"Vars"`
	Collect        map[string]collectInstruction `json:"Collect"`
}

type gameStatVarDef struct {
	LegacyValue string
	Autofill    string `json:"Autofill"`
	Display     string `json:"Display"`
	Placeholder string `json:"Placeholder"`
}

func (v *gameStatVarDef) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v.LegacyValue = strings.TrimSpace(s)
		return nil
	}
	type alias gameStatVarDef
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*v = gameStatVarDef(obj)
	v.Autofill = strings.TrimSpace(v.Autofill)
	v.Display = strings.TrimSpace(v.Display)
	v.Placeholder = strings.TrimSpace(v.Placeholder)
	return nil
}

type GameStatVarSpecDTO struct {
	Autofill    string `json:"autofill"`
	Display     string `json:"display"`
	Placeholder string `json:"placeholder"`
}

type displayRangeEntry struct {
	Min   *float64 `json:"min"`
	Max   *float64 `json:"max"`
	Value string   `json:"value"`
}

// displayPlaceholderRule maps a numeric value (from the collected metric, default %x%) into a token
// like %fill% inside DisplayAs. Ranges are checked in order; first inclusive match wins.
type displayPlaceholderRule struct {
	Key     string              `json:"key"`
	From    string              `json:"from"` // "x" (default) — parse as float from collected value
	Ranges  []displayRangeEntry `json:"ranges"`
	Default string              `json:"default"`
}

type collectInstruction struct {
	Source              string                   `json:"Source"`
	Path                string                   `json:"Path"`
	FallbackPaths       []string                 `json:"FallbackPaths"`
	Reducer             string                   `json:"Reducer"`
	ReducerOptions      map[string]any           `json:"ReducerOptions"`
	Pipeline            []string                 `json:"Pipeline"`
	XPath               string                   `json:"XPath"`
	Select              string                   `json:"Select"`
	SelectFunc          string                   `json:"SelectFunc"`
	DisplayAs           string                   `json:"DisplayAs"`
	DisplayPlaceholders []displayPlaceholderRule `json:"DisplayPlaceholders"`
	// DisplayFormat styles the value for %x_fmt% (e.g. commaNumber -> 13000 as 13,000). %x% stays the raw collected value (or data URI for imagedownload).
	DisplayFormat   string `json:"DisplayFormat"`
	ToggleText      string `json:"ToggleText"`
	SelectAttribute string `json:"SelectAttribute"`
	SpecialType     string `json:"SpecialType"`
	NoDisplayIf     string `json:"NoDisplayIf"`
	Icon            string `json:"Icon"`
}

type userGameStat struct {
	Vars          map[string]string `json:"Vars"`
	Collected     map[string]string `json:"Collected"`
	HiddenMetrics []string          `json:"HiddenMetrics"`
	LastUpdated   time.Time         `json:"LastUpdated"`
}

type HiddenMetricToggleDTO struct {
	Hidden     bool   `json:"hidden"`
	ToggleText string `json:"toggleText"`
}

type StatValueAndIconDTO struct {
	StatValue       string `json:"statValue"`
	IndicatorMarkup string `json:"indicatorMarkup"`
}

type gameStatsManager struct {
	mu          sync.Mutex
	loaded      bool
	defs        map[string]gameDefinition
	compat      map[string][]string
	cacheByGame map[string]map[string]userGameStat
}

var gameStatsState = &gameStatsManager{
	defs:        map[string]gameDefinition{},
	compat:      map[string][]string{},
	cacheByGame: map[string]map[string]userGameStat{},
}

var gameStatsLog = slog.Default().With("component", "game-stats")

func seedEmbeddedGameStats() {
	if len(embeddedGameStatsJSON) == 0 {
		return
	}
	root, err := paths.DataRoot()
	if err != nil {
		return
	}
	dest := filepath.Join(root, "GameStats.json")
	_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	payload := append([]byte(nil), embeddedGameStatsJSON...)
	_ = fsutil.WriteFileAtomic(dest, payload, 0o644)
	gameStatsLog.Debug("seeded embedded GameStats.json", "dest", dest, "bytes", len(payload))
}

func (m *gameStatsManager) ensureLoadedLocked() error {
	if m.loaded {
		return nil
	}
	seedEmbeddedGameStats()
	cfgPath, err := resolveGameStatsConfigPath()
	if err != nil {
		return err
	}
	gameStatsLog.Debug("loading game stats config", "path", cfgPath)
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", cfgPath, err)
	}
	var cfg gameStatsFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", cfgPath, err)
	}
	if dataRoot, derr := paths.DataRoot(); derr == nil {
		customPath := filepath.Join(dataRoot, "GameStats.custom.json")
		customRaw, err := os.ReadFile(customPath)
		if err == nil {
			if err := mergeGameStatsCustom(&cfg, customRaw); err != nil {
				return err
			}
			gameStatsLog.Debug("merged GameStats.custom.json", "path", customPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", customPath, err)
		}
	}
	m.defs = cfg.StatsDefinitions
	if m.defs == nil {
		m.defs = map[string]gameDefinition{}
	}
	m.compat = cfg.PlatformCompatibilities
	if m.compat == nil {
		m.compat = map[string][]string{}
	}
	for game, def := range m.defs {
		if def.Vars == nil {
			def.Vars = map[string]gameStatVarDef{}
		}
		if def.Collect == nil {
			def.Collect = map[string]collectInstruction{}
		}
		for key, ci := range def.Collect {
			if strings.TrimSpace(ci.DisplayAs) == "" {
				ci.DisplayAs = "%x%"
			}
			if strings.TrimSpace(ci.ToggleText) == "" {
				ci.ToggleText = key
			}
			def.Collect[key] = ci
		}
		m.defs[game] = def
		if _, ok := m.cacheByGame[game]; !ok {
			m.cacheByGame[game] = map[string]userGameStat{}
		}
		if err := m.loadGameCacheLocked(game); err != nil {
			return err
		}
		gameStatsLog.Debug("loaded game definition", "game", game, "vars", len(def.Vars), "collect", len(def.Collect))
	}
	m.loaded = true
	gameStatsLog.Info("game stats config loaded", "games", len(m.defs), "platformCompat", len(m.compat))
	return nil
}

func resolveGameStatsConfigPath() (string, error) {
	seedEmbeddedGameStats()
	if dataRoot, derr := paths.DataRoot(); derr == nil {
		p := filepath.Join(dataRoot, "GameStats.json")
		if fileExists(p) {
			return p, nil
		}
	}
	exeDir, eerr := platform.ResolveExeDir()
	if eerr == nil {
		p := filepath.Join(exeDir, "GameStats.json")
		if fileExists(p) {
			return p, nil
		}
	}
	if wd, werr := os.Getwd(); werr == nil {
		p := filepath.Join(wd, "GameStats.json")
		if fileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("GameStats.json not found in data dir, exe dir, or current dir")
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func gameStatsCacheRoot() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "StatsCache"), nil
}

func gameStatsCachePath(game string) (string, error) {
	root, err := gameStatsCacheRoot()
	if err != nil {
		return "", err
	}
	safe := paths.SanitizePathSegment(game)
	if safe == "" {
		safe = "game"
	}
	return filepath.Join(root, safe+".json"), nil
}

func (m *gameStatsManager) loadGameCacheLocked(game string) error {
	p, err := gameStatsCachePath(game)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			if m.cacheByGame[game] == nil {
				m.cacheByGame[game] = map[string]userGameStat{}
			}
			return nil
		}
		return fmt.Errorf("read %s: %w", p, err)
	}
	var cached map[string]userGameStat
	if err := json.Unmarshal(data, &cached); err != nil {
		// Corrupt cache should not block normal usage.
		m.cacheByGame[game] = map[string]userGameStat{}
		return nil
	}
	if cached == nil {
		cached = map[string]userGameStat{}
	}
	for accountID, row := range cached {
		if row.Vars == nil {
			row.Vars = map[string]string{}
		}
		if row.Collected == nil {
			row.Collected = map[string]string{}
		}
		row.HiddenMetrics = normalizeHiddenMetrics(row.HiddenMetrics, m.defs[game].Collect)
		cached[accountID] = row
	}
	m.cacheByGame[game] = cached
	gameStatsLog.Debug("loaded game stats cache", "game", game, "accounts", len(cached), "path", p)
	return nil
}

func (m *gameStatsManager) saveGameCacheLocked(game string) error {
	p, err := gameStatsCachePath(game)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
	}
	payload := m.cacheByGame[game]
	if payload == nil {
		payload = map[string]userGameStat{}
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	gameStatsLog.Debug("saving game stats cache", "game", game, "accounts", len(payload), "path", p)
	return fsutil.WriteFileAtomic(p, b, 0o644)
}

func normalizeHiddenMetrics(hidden []string, collect map[string]collectInstruction) []string {
	if len(hidden) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(hidden))
	for _, x := range hidden {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		if _, ok := collect[x]; !ok {
			continue
		}
		k := strings.ToLower(x)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func cloneStringMap(src map[string]string) map[string]string {
	dst := map[string]string{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneUserGameStat(u userGameStat) userGameStat {
	return userGameStat{
		Vars:          cloneStringMap(u.Vars),
		Collected:     cloneStringMap(u.Collected),
		HiddenMetrics: append([]string(nil), u.HiddenMetrics...),
		LastUpdated:   u.LastUpdated,
	}
}

func gameStatVarAutofillExpr(v gameStatVarDef) string {
	if strings.TrimSpace(v.Autofill) != "" {
		return strings.TrimSpace(v.Autofill)
	}
	return strings.TrimSpace(v.LegacyValue)
}

func gameStatVarDisplay(v gameStatVarDef, key string) string {
	if strings.TrimSpace(v.Display) != "" {
		return strings.TrimSpace(v.Display)
	}
	base := strings.TrimSpace(v.LegacyValue)
	if base == "" {
		return strings.TrimSpace(key)
	}
	return base
}

func gameStatVarPlaceholder(v gameStatVarDef) string {
	if strings.TrimSpace(v.Placeholder) != "" {
		return strings.TrimSpace(v.Placeholder)
	}
	base := strings.TrimSpace(v.LegacyValue)
	i := strings.Index(base, "[")
	if i >= 0 && strings.HasSuffix(base, "]") {
		return strings.TrimSpace(strings.TrimSuffix(base[i+1:], "]"))
	}
	return ""
}

func gameStatVarDefsToAutofillMap(defs map[string]gameStatVarDef) map[string]string {
	out := map[string]string{}
	for k, v := range defs {
		out[k] = gameStatVarAutofillExpr(v)
	}
	return out
}

func (b *BasicService) GetAvailableGames(platformName string) ([]string, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	row := append([]string(nil), gameStatsState.compat[strings.TrimSpace(platformName)]...)
	sort.Strings(row)
	return row, nil
}

func (b *BasicService) GetEnabledGames(platformName, accountID string) ([]string, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	accountID = strings.TrimSpace(accountID)
	var out []string
	for _, game := range gameStatsState.compat[strings.TrimSpace(platformName)] {
		if _, ok := gameStatsState.cacheByGame[game][accountID]; ok {
			out = append(out, game)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (b *BasicService) GetDisabledGames(platformName, accountID string) ([]string, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	accountID = strings.TrimSpace(accountID)
	var out []string
	for _, game := range gameStatsState.compat[strings.TrimSpace(platformName)] {
		if _, ok := gameStatsState.cacheByGame[game][accountID]; !ok {
			out = append(out, game)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (b *BasicService) GetRequiredVars(game string) (map[string]string, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	def, ok := gameStatsState.defs[strings.TrimSpace(game)]
	if !ok {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for key, spec := range def.Vars {
		label := gameStatVarDisplay(spec, key)
		placeholder := gameStatVarPlaceholder(spec)
		if placeholder != "" {
			out[key] = label + " [" + placeholder + "]"
			continue
		}
		auto := gameStatVarAutofillExpr(spec)
		if auto != "" {
			out[key] = auto
			continue
		}
		out[key] = label
	}
	return out, nil
}

func (b *BasicService) GetRequiredVarSpecs(game string) (map[string]GameStatVarSpecDTO, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	def, ok := gameStatsState.defs[strings.TrimSpace(game)]
	if !ok {
		return map[string]GameStatVarSpecDTO{}, nil
	}
	out := map[string]GameStatVarSpecDTO{}
	for key, spec := range def.Vars {
		out[key] = GameStatVarSpecDTO{
			Autofill:    gameStatVarAutofillExpr(spec),
			Display:     gameStatVarDisplay(spec, key),
			Placeholder: gameStatVarPlaceholder(spec),
		}
	}
	return out, nil
}

func (b *BasicService) GetExistingVars(game, accountID string) (map[string]string, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	game = strings.TrimSpace(game)
	accountID = strings.TrimSpace(accountID)
	row, ok := gameStatsState.cacheByGame[game][accountID]
	if !ok {
		return map[string]string{}, nil
	}
	return cloneStringMap(row.Vars), nil
}

func (b *BasicService) GetAllMetrics(game string) (map[string]string, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	game = strings.TrimSpace(game)
	def, ok := gameStatsState.defs[game]
	if !ok {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for key, ci := range def.Collect {
		txt := strings.TrimSpace(ci.ToggleText)
		if txt == "" {
			txt = key
		}
		out[key] = txt
	}
	return out, nil
}

func (b *BasicService) GetHiddenMetrics(game, accountID string) (map[string]HiddenMetricToggleDTO, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	game = strings.TrimSpace(game)
	accountID = strings.TrimSpace(accountID)
	def, ok := gameStatsState.defs[game]
	if !ok {
		return map[string]HiddenMetricToggleDTO{}, nil
	}
	hidden := map[string]struct{}{}
	if row, ok := gameStatsState.cacheByGame[game][accountID]; ok {
		for _, key := range row.HiddenMetrics {
			hidden[key] = struct{}{}
		}
	}
	out := map[string]HiddenMetricToggleDTO{}
	for key, ci := range def.Collect {
		_, isHidden := hidden[key]
		txt := strings.TrimSpace(ci.ToggleText)
		if txt == "" {
			txt = key
		}
		out[key] = HiddenMetricToggleDTO{
			Hidden:     isHidden,
			ToggleText: txt,
		}
	}
	return out, nil
}

func (b *BasicService) GetResolvedGameStatVars(platformName, game, accountID string) (map[string]string, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	game = strings.TrimSpace(game)
	accountID = strings.TrimSpace(accountID)
	platformName = strings.TrimSpace(platformName)
	if game == "" || accountID == "" {
		return map[string]string{}, nil
	}
	def, ok := gameStatsState.defs[game]
	if !ok {
		return map[string]string{}, nil
	}
	idf, err := readIdsFile(platformName)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(idf.IDs[accountID])
	display := username
	if display == "" {
		display = accountID
	}
	stored := map[string]string{}
	if row, ok := gameStatsState.cacheByGame[game][accountID]; ok && row.Vars != nil {
		stored = row.Vars
	}
	ctx := GameStatVarContext{AccountID: accountID, AccountUsername: display, Username: username}
	return ResolveGameStatsVarTemplates(gameStatVarDefsToAutofillMap(def.Vars), stored, ctx), nil
}

func (b *BasicService) SetGameVars(platformName, game, accountID string, vars map[string]string, hiddenMetrics []string) (bool, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return false, err
	}
	platformName = strings.TrimSpace(platformName)
	game = strings.TrimSpace(game)
	accountID = strings.TrimSpace(accountID)
	if accountID == "" || game == "" {
		return false, nil
	}
	def, ok := gameStatsState.defs[game]
	if !ok {
		return false, nil
	}
	if gameStatsState.cacheByGame[game] == nil {
		gameStatsState.cacheByGame[game] = map[string]userGameStat{}
	}
	rows := gameStatsState.cacheByGame[game]
	prev, hadPrev := rows[accountID]
	var prevCopy userGameStat
	if hadPrev {
		prevCopy = cloneUserGameStat(prev)
	}
	existing := prev
	if existing.Vars == nil {
		existing.Vars = map[string]string{}
	}
	if existing.Collected == nil {
		existing.Collected = map[string]string{}
	}
	existing.Vars = cloneStringMap(vars)
	existing.HiddenMetrics = normalizeHiddenMetrics(hiddenMetrics, def.Collect)
	if existing.LastUpdated.IsZero() {
		existing.LastUpdated = time.Now()
	}
	gameStatsState.cacheByGame[game][accountID] = existing
	if err := gameStatsState.refreshFromWebLocked(platformName, game, accountID); err != nil {
		if hadPrev {
			gameStatsState.cacheByGame[game][accountID] = prevCopy
		} else {
			delete(gameStatsState.cacheByGame[game], accountID)
		}
		if saveErr := gameStatsState.saveGameCacheLocked(game); saveErr != nil {
			return false, saveErr
		}
		gameStatsLog.Warn("game stats refresh after save failed", "platform", platformName, "game", game, "accountID", accountID, "err", err)
		return false, err
	}
	gameStatsLog.Info("game stats vars updated", "platform", platformName, "game", game, "accountID", accountID, "vars", len(existing.Vars), "hiddenMetrics", len(existing.HiddenMetrics))
	return true, nil
}

func (b *BasicService) DisableGame(game, accountID string) error {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return err
	}
	game = strings.TrimSpace(game)
	accountID = strings.TrimSpace(accountID)
	rows := gameStatsState.cacheByGame[game]
	if rows == nil {
		return nil
	}
	if _, ok := rows[accountID]; !ok {
		return nil
	}
	delete(rows, accountID)
	gameStatsState.cacheByGame[game] = rows
	gameStatsLog.Info("game stats disabled", "game", game, "accountID", accountID)
	return gameStatsState.saveGameCacheLocked(game)
}

func (b *BasicService) GetUserStatsAllGamesMarkup(platformName, accountID string) (map[string]map[string]StatValueAndIconDTO, error) {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return nil, err
	}
	accountID = strings.TrimSpace(accountID)
	out := map[string]map[string]StatValueAndIconDTO{}
	for _, game := range gameStatsState.compat[strings.TrimSpace(platformName)] {
		def, ok := gameStatsState.defs[game]
		if !ok {
			continue
		}
		row, ok := gameStatsState.cacheByGame[game][accountID]
		if !ok {
			continue
		}
		hidden := map[string]struct{}{}
		for _, k := range row.HiddenMetrics {
			hidden[k] = struct{}{}
		}
		statsByKey := map[string]StatValueAndIconDTO{}
		for key, value := range row.Collected {
			if _, ok := def.Collect[key]; !ok {
				// Ignore stale cached keys no longer present in current definition.
				continue
			}
			if _, skip := hidden[key]; skip {
				continue
			}
			indicator := strings.TrimSpace(def.Collect[key].Icon)
			if indicator == "" && strings.TrimSpace(def.Indicator) != "" {
				indicator = "<sup>" + def.Indicator + "</sup>"
			}
			statsByKey[key] = StatValueAndIconDTO{
				StatValue:       value,
				IndicatorMarkup: indicator,
			}
		}
		if len(statsByKey) > 0 {
			out[game] = statsByKey
		}
	}
	return out, nil
}

func (m *gameStatsManager) refreshFromWebLocked(platformName, game, accountID string) error {
	if appclient.IsOfflineMode() {
		return appclient.ErrOfflineMode
	}
	platformName = strings.TrimSpace(platformName)
	game = strings.TrimSpace(game)
	accountID = strings.TrimSpace(accountID)
	if game == "" || accountID == "" {
		return fmt.Errorf("missing game or account id")
	}
	def, ok := m.defs[game]
	if !ok {
		return fmt.Errorf("unknown game stats definition")
	}
	row, ok := m.cacheByGame[game][accountID]
	if !ok {
		return fmt.Errorf("stats not enabled for this account")
	}
	idf, err := readIdsFile(platformName)
	if err != nil {
		return err
	}
	username := strings.TrimSpace(idf.IDs[accountID])
	display := username
	if display == "" {
		display = accountID
	}
	ctx := GameStatVarContext{AccountID: accountID, AccountUsername: display, Username: username}
	resolved := ResolveGameStatsVarTemplates(gameStatVarDefsToAutofillMap(def.Vars), row.Vars, ctx)
	urlStr := substituteGameStatsURL(def.URL, resolved)
	gameStatsLog.Info("refresh game stats begin", "platform", platformName, "game", game, "accountID", accountID, "url", urlStr)
	reqCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rawHTML, err := fetchGameStatsHTML(reqCtx, urlStr, def.RequestCookies)
	if err != nil {
		gameStatsLog.Warn("refresh game stats fetch failed", "platform", platformName, "game", game, "accountID", accountID, "url", urlStr, "err", err)
		return err
	}
	doc, err := htmlquery.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		return err
	}
	collected, err := collectStatsFromHTML(platformName, accountID, def, doc, rawHTML)
	if err != nil {
		return err
	}
	if len(collected) == 0 {
		writeGameStatsDebugHTML(accountID, game, rawHTML)
		// Keep the game enabled and preserve vars/hidden metrics; only clear collected values.
		row.Collected = map[string]string{}
		m.cacheByGame[game][accountID] = row
		_ = m.saveGameCacheLocked(game)
		gameStatsLog.Warn("refresh game stats extracted no rows", "platform", platformName, "game", game, "accountID", accountID, "htmlBytes", len(rawHTML))
		return fmt.Errorf("no statistics extracted (saved debug HTML under DataRoot/temp)")
	}
	row.Collected = collected
	row.LastUpdated = time.Now()
	row.Vars = cloneStringMap(row.Vars)
	m.cacheByGame[game][accountID] = row
	gameStatsLog.Info("refresh game stats success", "platform", platformName, "game", game, "accountID", accountID, "collected", len(collected))
	return m.saveGameCacheLocked(game)
}

// RefreshGameStats downloads game statistics HTML for one enabled game and updates the cache.
func (b *BasicService) RefreshGameStats(platformName, game, accountID string) error {
	gameStatsState.mu.Lock()
	defer gameStatsState.mu.Unlock()
	if err := gameStatsState.ensureLoadedLocked(); err != nil {
		return err
	}
	err := gameStatsState.refreshFromWebLocked(platformName, game, accountID)
	if err != nil && isGameStatsResourceNotFound(err) {
		g := strings.TrimSpace(game)
		acct := strings.TrimSpace(accountID)
		if rows := gameStatsState.cacheByGame[g]; rows != nil {
			delete(rows, acct)
			gameStatsState.cacheByGame[g] = rows
			if saveErr := gameStatsState.saveGameCacheLocked(g); saveErr != nil {
				return saveErr
			}
			gameStatsLog.Info("game stats disabled after not-found response", "game", g, "accountID", acct)
		}
	}
	return err
}
