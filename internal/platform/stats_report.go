package platform

import (
	"time"

	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/winutil"
)

// StatsSwitcherRow is one platform block in the stats report (bindings / UI).
type StatsSwitcherRow struct {
	Platform            string `json:"platform"`
	Accounts            int    `json:"accounts"`
	Switches            int    `json:"switches"`
	UniqueDays          int    `json:"uniqueDays"`
	GameShortcuts       int    `json:"gameShortcuts"`
	GameShortcutsHotbar int    `json:"gameShortcutsHotbar"`
	GamesLaunched       int    `json:"gamesLaunched"`
	Tags                int    `json:"tags"`
	TaggedAccounts      int    `json:"taggedAccounts"`
	FirstActive         string `json:"firstActive"`
	LastActive          string `json:"lastActive"`
}

// StatsPageRow is one page-visit aggregate in the stats report.
type StatsPageRow struct {
	Path         string `json:"path"`
	Visits       int    `json:"visits"`
	TotalTimeSec int    `json:"totalTimeSec"`
}

// StatsReport is returned to the SPA for the statistics modal.
type StatsReport struct {
	ShareEnabled       bool              `json:"shareEnabled"`
	OsDisplay          string            `json:"osDisplay"`
	FirstLaunch        string            `json:"firstLaunch"`
	LaunchCount        int               `json:"launchCount"`
	CrashCount         int               `json:"crashCount"`
	MostUsedPlatform   string            `json:"mostUsedPlatform"`
	TotalTimeInAppSec  int               `json:"totalTimeInAppSec"`
	TotalSwitches      int               `json:"totalSwitches"`
	TotalGamesLaunched  int               `json:"totalGamesLaunched"`
	TotalTags           int               `json:"totalTags"`
	TotalTaggedAccounts int               `json:"totalTaggedAccounts"`
	UniqueDaysSwitched  int               `json:"uniqueDaysSwitched"`
	UUID               string            `json:"uuid"`
	LastUpload         string            `json:"lastUpload"`
	Switchers          []StatsSwitcherRow `json:"switchers"`
	Pages              []StatsPageRow    `json:"pages"`
}

func formatStatsDateTime(t time.Time) string {
	if t.IsZero() {
		return "0001-01-01 00:00:00"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func assembleStatsReport(data stats.ReportData, shareEnabled bool) StatsReport {
	sw := make([]StatsSwitcherRow, 0, len(data.Switchers))
	for _, r := range data.Switchers {
		sw = append(sw, StatsSwitcherRow{
			Platform:            r.Platform,
			Accounts:            r.Accounts,
			Switches:            r.Switches,
			UniqueDays:          r.UniqueDays,
			GameShortcuts:       r.GameShortcuts,
			GameShortcutsHotbar: r.GameShortcutsHotbar,
			GamesLaunched:       r.GamesLaunched,
			Tags:                r.Tags,
			TaggedAccounts:      r.TaggedAccounts,
			FirstActive:         formatStatsDateTime(r.FirstActive),
			LastActive:          formatStatsDateTime(r.LastActive),
		})
	}
	pg := make([]StatsPageRow, 0, len(data.Pages))
	for _, p := range data.Pages {
		pg = append(pg, StatsPageRow{
			Path:         p.Path,
			Visits:       p.Visits,
			TotalTimeSec: p.TotalTime,
		})
	}
	return StatsReport{
		ShareEnabled:       shareEnabled,
		OsDisplay:          winutil.OSDisplayString(),
		FirstLaunch:        formatStatsDateTime(data.FirstLaunch),
		LaunchCount:        data.LaunchCount,
		CrashCount:         data.CrashCount,
		MostUsedPlatform:   data.MostUsedPlatform,
		TotalTimeInAppSec:  data.TotalTimeInAppSec,
		TotalSwitches:      data.TotalSwitches,
		TotalGamesLaunched:  data.TotalGamesLaunched,
		TotalTags:           data.TotalTags,
		TotalTaggedAccounts: data.TotalTaggedAccounts,
		UniqueDaysSwitched:  data.UniqueDaysSwitched,
		UUID:               data.UUID,
		LastUpload:         formatStatsDateTime(data.LastUpload),
		Switchers:          sw,
		Pages:              pg,
	}
}
