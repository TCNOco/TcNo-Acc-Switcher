package stats

import (
	"sort"
	"strings"
	"time"
)

// ReportSwitcherEntry is one platform row for the stats UI (excluding aggregate _Total).
type ReportSwitcherEntry struct {
	Platform             string
	Accounts             int
	Switches             int
	UniqueDays           int
	GameShortcuts        int
	GameShortcutsHotbar  int
	GamesLaunched        int
	FirstActive          time.Time
	LastActive           time.Time
}

// ReportPageEntry is one tracked route for the stats UI (excluding aggregate _Total).
type ReportPageEntry struct {
	Path      string
	Visits    int
	TotalTime int
}

// ReportData is a snapshot of stored statistics for presentation.
type ReportData struct {
	FirstLaunch          time.Time
	LaunchCount          int
	CrashCount           int
	MostUsedPlatform     string
	TotalTimeInAppSec    int
	TotalSwitches        int
	TotalGamesLaunched   int
	UniqueDaysSwitched   int
	UUID                 string
	LastUpload           time.Time
	Switchers            []ReportSwitcherEntry
	Pages                []ReportPageEntry
}

// GetReportData loads statistics, refreshes totals, and returns sorted slices for the UI.
func GetReportData() (ReportData, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return ReportData{}, err
	}
	generateTotalsLocked()

	var sw []ReportSwitcherEntry
	for k, v := range state.SwitcherStats {
		if k == "_Total" {
			continue
		}
		sw = append(sw, ReportSwitcherEntry{
			Platform:             k,
			Accounts:             v.Accounts,
			Switches:             v.Switches,
			UniqueDays:           v.UniqueDays,
			GameShortcuts:        v.GameShortcuts,
			GameShortcutsHotbar:  v.GameShortcutsHotbar,
			GamesLaunched:        v.GamesLaunched,
			FirstActive:          v.FirstActive,
			LastActive:           v.LastActive,
		})
	}
	sort.Slice(sw, func(i, j int) bool {
		if sw[i].Switches != sw[j].Switches {
			return sw[i].Switches > sw[j].Switches
		}
		return strings.ToLower(sw[i].Platform) < strings.ToLower(sw[j].Platform)
	})

	var pg []ReportPageEntry
	for k, v := range state.PageStats {
		if k == "_Total" {
			continue
		}
		pg = append(pg, ReportPageEntry{Path: k, Visits: v.Visits, TotalTime: v.TotalTime})
	}
	sort.Slice(pg, func(i, j int) bool {
		if pg[i].TotalTime != pg[j].TotalTime {
			return pg[i].TotalTime > pg[j].TotalTime
		}
		return strings.ToLower(pg[i].Path) < strings.ToLower(pg[j].Path)
	})

	tot := state.SwitcherStats["_Total"]
	pageTot := state.PageStats["_Total"]

	return ReportData{
		FirstLaunch:        state.FirstLaunch,
		LaunchCount:        state.LaunchCount,
		CrashCount:         state.CrashCount,
		MostUsedPlatform:   state.MostUsedPlatform,
		TotalTimeInAppSec:  pageTot.TotalTime,
		TotalSwitches:      tot.Switches,
		TotalGamesLaunched: tot.GamesLaunched,
		UniqueDaysSwitched: tot.UniqueDays,
		UUID:               state.Uuid,
		LastUpload:         state.LastUpload,
		Switchers:          sw,
		Pages:              pg,
	}, nil
}

// ResetStatistics replaces stored statistics with a fresh file (new UUID, counters cleared).
func ResetStatistics() error {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return err
	}
	state = defaultStats()
	return saveLocked()
}
