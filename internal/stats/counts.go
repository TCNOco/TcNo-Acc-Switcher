package stats

import "strings"

// GetPlatformAccountCounts returns stored per-platform account totals (excludes aggregate _Total).
func GetPlatformAccountCounts() (map[string]int, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]int)
	for k, v := range state.SwitcherStats {
		if k == "_Total" {
			continue
		}
		if v.Accounts > 0 {
			out[k] = v.Accounts
		}
	}
	return out, nil
}

// LookupPlatformAccountCount returns a stored account total when the platform exists in Statistics.json.
func LookupPlatformAccountCount(platformName string) (int, bool) {
	platformName = strings.TrimSpace(platformName)
	if platformName == "" || platformName == "_Total" {
		return 0, false
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return 0, false
	}
	row, ok := state.SwitcherStats[platformName]
	if !ok {
		return 0, false
	}
	return row.Accounts, true
}

// LookupPlatformTagCount returns stored tag & tagged-account counts when the platform exists in Statistics.json.
func LookupPlatformTagCount(platformName string) (tags int, taggedAccounts int, ok bool) {
	platformName = strings.TrimSpace(platformName)
	if platformName == "" || platformName == "_Total" {
		return 0, 0, false
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return 0, 0, false
	}
	row, found := state.SwitcherStats[platformName]
	if !found {
		return 0, 0, false
	}
	return row.Tags, row.TaggedAccounts, true
}
