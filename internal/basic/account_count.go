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
