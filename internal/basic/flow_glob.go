package basic

import (
	"path/filepath"
	"strings"
)

func hasGlobPattern(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func globDestinationRoot(destRoot, cacheRel string) string {
	cacheRel = strings.TrimSpace(cacheRel)
	dst := filepath.Join(destRoot, filepath.FromSlash(cacheRel))
	if strings.HasSuffix(cacheRel, "\\") || strings.HasSuffix(cacheRel, "/") {
		return dst
	}
	return filepath.Dir(dst)
}

func globPatternBaseDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "."
	}
	idx := strings.IndexAny(path, "*?[")
	if idx < 0 {
		return filepath.Dir(path)
	}
	prefix := path[:idx]
	if strings.HasSuffix(prefix, "\\") || strings.HasSuffix(prefix, "/") {
		return filepath.Clean(prefix)
	}
	return filepath.Dir(prefix)
}
