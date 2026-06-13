package basic

import "strings"

// CountSavedAccounts returns the number of accounts stored for a basic platform (from ids.json).
func CountSavedAccounts(platformKey string) int {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return 0
	}
	f, err := readIdsFile(platformKey)
	if err != nil || f.IDs == nil {
		return 0
	}
	return len(f.IDs)
}



// CountTags returns the number of tag definitions stored for a platform (from ids.json).
func CountTags(platformKey string) int {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return 0
	}
	f, err := readIdsFile(platformKey)
	if err != nil || f.Tags == nil {
		return 0
	}
	return len(f.Tags)
}

// CountTaggedAccounts returns the number of accounts with at least one tag (from ids.json).
func CountTaggedAccounts(platformKey string) int {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return 0
	}
	f, err := readIdsFile(platformKey)
	if err != nil || f.AccountTags == nil {
		return 0
	}
	return len(f.AccountTags)
}
