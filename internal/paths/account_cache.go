package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AccountCacheDir also recognizes the less restrictive directory names used by
// the .NET app. Keep those directories in place, including when encryption
// enumerates sessions, rather than making a second copy of saved login data.
func AccountCacheDir(platformKey, accountName string) (string, error) {
	base, err := LoginCacheDir(platformKey)
	if err != nil {
		return "", err
	}
	normalized := WindowsFileName(accountName, 200)
	if normalized == "" {
		normalized = "_"
	}
	current := filepath.Join(base, normalized)
	if accountName == normalized || !safeLegacyAccountName(accountName) {
		return current, nil
	}
	legacy := filepath.Join(base, accountName)
	info, err := os.Lstat(legacy)
	if os.IsNotExist(err) {
		return current, nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("legacy account cache is not a regular directory")
	}
	if _, err := os.Lstat(current); err == nil {
		return "", fmt.Errorf("both legacy and normalized account cache directories exist")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return legacy, nil
}

func safeLegacyAccountName(name string) bool {
	if name == "" || name == "." || name == ".." ||
		strings.TrimRight(name, " .") != name || strings.ContainsAny(name, `<>:"/\|?*`) {
		return false
	}
	for _, r := range name {
		if r < 32 {
			return false
		}
	}
	stem := strings.SplitN(name, ".", 2)[0]
	return !windowsReservedFileStem(stem)
}
