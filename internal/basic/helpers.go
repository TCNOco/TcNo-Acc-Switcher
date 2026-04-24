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

// parseJSONSelect splits JSON_SELECT_FIRST,::path::gjsonPath (first :: separates path from gjson path).
func parseJSONSelect(prefix, key string) (filePath, jsonPath string, ok bool) {
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, prefix) {
		return "", "", false
	}
	rest := strings.TrimSpace(key[len(prefix):])
	rest = strings.TrimPrefix(rest, ",")
	rest = strings.TrimPrefix(rest, "::")
	idx := strings.Index(rest, "::")
	if idx < 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+2:], true
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
