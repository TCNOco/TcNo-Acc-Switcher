package app

import (
	"strings"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/parallel"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/steam"
)

// RegisterStartupAccountCounts wires per-platform account and tag totals for GetStartup skeleton hints.
func RegisterStartupAccountCounts() {
	platform.SetStartupCountResolver(resolveStartupCounts)
}

type startupPlatformCounts struct {
	name     string
	accounts int
	tags     platform.PlatformTagCountInfo
}

// resolveStartupCounts gathers the account and tag totals GetStartup shows as
// skeleton hints. It runs before the window is drawn, so it visits every
// platform in Platforms.json whether or not the user has accounts on it.
//
// Platforms are independent and dominated by the ids.json read, so they resolve
// concurrently with one read each.
func resolveStartupCounts(platformNames []string, statsEnabled bool) (map[string]int, map[string]platform.PlatformTagCountInfo) {
	accounts := make(map[string]int, len(platformNames))
	tagCounts := make(map[string]platform.PlatformTagCountInfo, len(platformNames))
	if security.AppLocked() {
		return accounts, tagCounts
	}

	names := make([]string, 0, len(platformNames))
	for _, name := range platformNames {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return accounts, tagCounts
	}

	results := make([]startupPlatformCounts, len(names))
	parallel.ForEachIndex(len(names), func(i int) {
		results[i] = resolvePlatformCounts(names[i], statsEnabled)
	})

	for _, r := range results {
		accounts[r.name] = r.accounts
		tagCounts[r.name] = r.tags
	}
	return accounts, tagCounts
}

func resolvePlatformCounts(name string, statsEnabled bool) startupPlatformCounts {
	out := startupPlatformCounts{name: name}

	// Steam keeps its accounts outside ids.json, but its tags do not.
	isSteam := strings.EqualFold(name, steam.PlatformKey)

	accountsKnown := false
	if statsEnabled {
		if count, ok := stats.LookupPlatformAccountCount(name); ok {
			out.accounts = count
			accountsKnown = true
		}
	}
	if !accountsKnown && isSteam {
		out.accounts = steam.CountSavedAccounts()
		accountsKnown = true
	}

	c := basic.CountsFor(name)
	if !accountsKnown {
		out.accounts = c.Accounts
	}
	out.tags = platform.PlatformTagCountInfo{
		TagCount:           c.Tags,
		TaggedAccountCount: c.TaggedAccounts,
	}
	return out
}
