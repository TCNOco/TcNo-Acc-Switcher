package shortcuts

import (
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/exeicon"
)

func removeShortcutExt(name string) string {
	b := filepath.Base(strings.TrimSpace(name))
	switch strings.ToLower(filepath.Ext(b)) {
	case ".lnk", ".url":
		return strings.TrimSuffix(b, filepath.Ext(b))
	default:
		return b
	}
}

func iconPublicURL(platformKey, fileName string) string {
	stem := removeShortcutExt(filepath.Base(fileName))
	if stem == "" {
		return ""
	}
	return "/img/shortcuts/" + exeicon.SafeFolderName(platformKey) + "/" + strings.ToLower(stem) + ".png"
}

func isShortcutFile(name string) bool {
	low := strings.ToLower(name)
	return strings.HasSuffix(low, ".lnk") || strings.HasSuffix(low, ".url")
}

func ignoredName(orig string) string {
	low := strings.ToLower(orig)
	if strings.HasSuffix(low, ".lnk") {
		return strings.TrimSuffix(orig, filepath.Ext(orig)) + "_ignored.lnk"
	}
	if strings.HasSuffix(low, ".url") {
		return strings.TrimSuffix(orig, filepath.Ext(orig)) + "_ignored.url"
	}
	return orig + "_ignored"
}

func ignoreSet(names []string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			m[strings.ToLower(n)] = struct{}{}
		}
	}
	return m
}
