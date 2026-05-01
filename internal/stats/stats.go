package stats

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"

	"github.com/google/uuid"
)

const (
	fileName  = "Statistics.json"
	uploadURL = "https://tcno.co/Projects/AccSwitcher/api/stats/"
)

type PageStat struct {
	TotalTime int `json:"TotalTime"`
	Visits    int `json:"Visits"`
}

type SwitcherStat struct {
	Accounts            int       `json:"Accounts"`
	Switches            int       `json:"Switches"`
	UniqueDays          int       `json:"UniqueDays"`
	GameShortcuts       int       `json:"GameShortcuts"`
	GameShortcutsHotbar int       `json:"GameShortcutsHotbar"`
	GamesLaunched       int       `json:"GamesLaunched"`
	FirstActive         time.Time `json:"FirstActive"`
	LastActive          time.Time `json:"LastActive"`
}

type AppStats struct {
	Uuid             string                  `json:"Uuid"`
	LastUpload       time.Time               `json:"LastUpload"`
	OperatingSystem  string                  `json:"OperatingSystem"`
	LaunchCount      int                     `json:"LaunchCount"`
	CrashCount       int                     `json:"CrashCount"`
	FirstLaunch      time.Time               `json:"FirstLaunch"`
	MostUsedPlatform string                  `json:"MostUsedPlatform"`
	PageStats        map[string]PageStat     `json:"PageStats"`
	SwitcherStats    map[string]SwitcherStat `json:"SwitcherStats"`
}

var (
	mu     sync.Mutex
	loaded bool
	state  AppStats
)

func defaultSwitcherStat() SwitcherStat {
	now := time.Now()
	return SwitcherStat{
		UniqueDays:  1,
		FirstActive: now,
		LastActive:  now,
	}
}

func defaultStats() AppStats {
	return AppStats{
		Uuid:             uuid.NewString(),
		OperatingSystem:  runtime.GOOS,
		FirstLaunch:      time.Now(),
		PageStats:        map[string]PageStat{"_Total": {}},
		SwitcherStats:    map[string]SwitcherStat{"_Total": defaultSwitcherStat()},
		MostUsedPlatform: "",
	}
}

func statsPath() (string, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(exeDir, fileName), nil
}

func ensureLoadedLocked() error {
	if loaded {
		return nil
	}
	path, err := statsPath()
	if err != nil {
		return err
	}
	data, err := osReadFile(path)
	if err != nil {
		if isNotExist(err) {
			state = defaultStats()
			loaded = true
			return saveLocked()
		}
		return err
	}
	next := defaultStats()
	if err := json.Unmarshal(data, &next); err != nil {
		state = defaultStats()
		loaded = true
		_ = saveLocked()
		return nil
	}
	if strings.TrimSpace(next.Uuid) == "" {
		next.Uuid = uuid.NewString()
	}
	if next.FirstLaunch.IsZero() {
		next.FirstLaunch = time.Now()
	}
	if strings.TrimSpace(next.OperatingSystem) == "" {
		next.OperatingSystem = runtime.GOOS
	}
	if next.PageStats == nil {
		next.PageStats = map[string]PageStat{}
	}
	if _, ok := next.PageStats["_Total"]; !ok {
		next.PageStats["_Total"] = PageStat{}
	}
	if next.SwitcherStats == nil {
		next.SwitcherStats = map[string]SwitcherStat{}
	}
	if _, ok := next.SwitcherStats["_Total"]; !ok {
		next.SwitcherStats["_Total"] = defaultSwitcherStat()
	}
	state = next
	loaded = true
	return nil
}

func saveLocked() error {
	generateTotalsLocked()
	path, err := statsPath()
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, out, 0o644)
}

func ensurePlatformLocked(platformName string) string {
	p := strings.TrimSpace(platformName)
	if p == "" {
		p = "Unknown"
	}
	if _, ok := state.SwitcherStats[p]; !ok {
		state.SwitcherStats[p] = defaultSwitcherStat()
	}
	return p
}

func generateTotalsLocked() {
	total := defaultSwitcherStat()
	total.UniqueDays = 0
	total.FirstActive = time.Time{}
	total.LastActive = time.Time{}

	mostUsedPlatform := ""
	mostUsedCount := 0
	for k, v := range state.SwitcherStats {
		if k == "_Total" {
			continue
		}
		total.Accounts += v.Accounts
		total.Switches += v.Switches
		total.UniqueDays += v.UniqueDays
		total.GameShortcuts += v.GameShortcuts
		total.GameShortcutsHotbar += v.GameShortcutsHotbar
		total.GamesLaunched += v.GamesLaunched
		if total.FirstActive.IsZero() || (!v.FirstActive.IsZero() && v.FirstActive.Before(total.FirstActive)) {
			total.FirstActive = v.FirstActive
		}
		if v.LastActive.After(total.LastActive) {
			total.LastActive = v.LastActive
		}
		if v.Switches > mostUsedCount {
			mostUsedCount = v.Switches
			mostUsedPlatform = k
		}
	}
	if total.FirstActive.IsZero() {
		now := time.Now()
		total.FirstActive = now
		total.LastActive = now
		total.UniqueDays = 1
	}
	state.SwitcherStats["_Total"] = total

	pageTotal := PageStat{}
	for k, v := range state.PageStats {
		if k == "_Total" {
			continue
		}
		pageTotal.TotalTime += v.TotalTime
		pageTotal.Visits += v.Visits
	}
	state.PageStats["_Total"] = pageTotal
	state.MostUsedPlatform = mostUsedPlatform
}

func IncrementLaunchCount() error {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	state.LaunchCount++
	return saveLocked()
}

func IncrementSwitches(platformName string) error {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	p := ensurePlatformLocked(platformName)
	row := state.SwitcherStats[p]
	row.Switches++
	now := time.Now()
	if row.LastActive.IsZero() {
		row.FirstActive = now
		row.LastActive = now
		row.UniqueDays = 1
	} else if row.LastActive.Year() != now.Year() || row.LastActive.Month() != now.Month() || row.LastActive.Day() != now.Day() {
		row.UniqueDays++
		row.LastActive = now
	}
	state.SwitcherStats[p] = row
	return saveLocked()
}

func TryUploadDaily(settings platform.AppSettings) error {
	if !settings.StatsEnabled || !settings.StatsShare || settings.OfflineMode {
		return nil
	}

	mu.Lock()
	if err := ensureLoadedLocked(); err != nil {
		mu.Unlock()
		return err
	}
	if state.LastUpload.Year() == time.Now().Year() &&
		state.LastUpload.Month() == time.Now().Month() &&
		state.LastUpload.Day() == time.Now().Day() {
		mu.Unlock()
		return nil
	}
	generateTotalsLocked()
	payloadBytes, err := json.Marshal(state)
	if err != nil {
		mu.Unlock()
		return err
	}
	uuid := state.Uuid
	mu.Unlock()

	resp, err := http.PostForm(uploadURL, url.Values{
		"uuid":      []string{uuid},
		"statsData": []string{string(payloadBytes)},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stats upload failed with status: %d", resp.StatusCode)
	}

	mu.Lock()
	defer mu.Unlock()
	state.LastUpload = time.Now()
	return saveLocked()
}

func MustTryUploadDaily(settings platform.AppSettings) {
	if err := TryUploadDaily(settings); err != nil {
		log.Printf("stats upload skipped/failed: %v", err)
	}
}

// Narrow wrappers keep imports in one file and simplify test replacement.
var (
	osReadFile = func(path string) ([]byte, error) { return os.ReadFile(path) }
	isNotExist = func(err error) bool { return os.IsNotExist(err) }
)
