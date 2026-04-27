package basic

import (
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

func loadDescriptor(raw []byte, platformKey string) (platform.Descriptor, error) {
	return platform.ParseDescriptor(raw, platformKey)
}

func stripREG(key string) string {
	k := strings.TrimSpace(key)
	if strings.HasPrefix(strings.ToUpper(k), "REG:") {
		return strings.TrimSpace(k[4:])
	}
	return k
}

func isREG(key string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(key)), "REG:")
}

func isJSONSelectFirst(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), "JSON_SELECT_FIRST")
}

func isJSONSelectLast(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), "JSON_SELECT_LAST")
}

func parseJSONSelectWithDelimiter(prefix, key string) (filePath, jsonPath, delimiter string, ok bool) {
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, prefix) {
		return "", "", "", false
	}
	rest := strings.TrimSpace(key[len(prefix):])
	firstSep := strings.Index(rest, "::")
	if firstSep < 0 {
		return "", "", "", false
	}
	delimiter = rest[:firstSep]
	rest = rest[firstSep+2:]
	secondSep := strings.Index(rest, "::")
	if secondSep < 0 {
		return "", "", "", false
	}
	filePath = rest[:secondSep]
	jsonPath = rest[secondSep+2:]
	return filePath, jsonPath, delimiter, true
}

func parseJSONSelect(prefix, key string) (filePath, jsonPath string, ok bool) {
	filePath, jsonPath, _, ok = parseJSONSelectWithDelimiter(prefix, key)
	return filePath, jsonPath, ok
}

func expandPlatformPath(s string, platformFolder string, ctx platform.PathTokenContext) string {
	ctx.PlatformFolder = platformFolder
	return platform.ExpandPathTokens(platform.ExpandWindowsPath(s), ctx)
}

func accountCacheDir(platformKey, accountName string) (string, error) {
	base, err := loginCacheRoot(platformKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, safeFileName(accountName)), nil
}

func safeFileName(s string) string {
	out := paths.WindowsFileName(s, 200)
	if out == "" {
		return "_"
	}
	return out
}
