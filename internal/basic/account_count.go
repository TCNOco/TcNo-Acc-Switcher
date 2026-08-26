package basic

import "strings"

// PlatformCounts holds every ids.json-derived total the startup hints ask for.
type PlatformCounts struct {
	Accounts       int
	Tags           int
	TaggedAccounts int
}

// CountsFor reads ids.json once and derives all three counts from it. Prefer it
// over the single-purpose helpers below when more than one count is wanted: each
// of those re-reads and re-parses the whole file.
func CountsFor(platformKey string) PlatformCounts {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return PlatformCounts{}
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return PlatformCounts{}
	}
	return PlatformCounts{
		Accounts:       len(f.IDs),
		Tags:           len(f.Tags),
		TaggedAccounts: len(f.AccountTags),
	}
}

// CountSavedAccounts returns the number of accounts stored for a basic platform (from ids.json).
func CountSavedAccounts(platformKey string) int {
	return CountsFor(platformKey).Accounts
}

// CountTags returns the number of tag definitions stored for a platform (from ids.json).
func CountTags(platformKey string) int {
	return CountsFor(platformKey).Tags
}

// CountTaggedAccounts returns the number of accounts with at least one tag (from ids.json).
func CountTaggedAccounts(platformKey string) int {
	return CountsFor(platformKey).TaggedAccounts
}
