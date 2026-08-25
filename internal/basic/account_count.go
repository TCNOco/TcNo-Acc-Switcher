package basic

import "strings"

// PlatformCounts holds every ids.json-derived total the startup hints ask for.
type PlatformCounts struct {
	Accounts       int
	Tags           int
	TaggedAccounts int
}

// CountsFor reads ids.json once and derives all three counts from it.
//
// Startup wants all three for every platform in Platforms.json. Asking through
// the single-purpose helpers below re-reads and re-parses the same file three
// times, and that read is ~70% syscall time on Windows, so collapsing it to one
// read per platform removes most of the cost.
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
