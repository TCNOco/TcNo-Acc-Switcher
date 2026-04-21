package exeicon

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"TcNo-Acc-Switcher/internal/winutil"
)

// SafeFolderName mirrors profileimage.PlatformFolder-style slug for paths.
func SafeFolderName(platformKey string) string {
	s := strings.TrimSpace(strings.ToLower(platformKey))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == ' ' || r == '/' || r == '\\':
			b.WriteRune('_')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "unknown"
	}
	return out
}

// PublicURL returns the URL path served from wwwroot (leading slash).
func PublicURL(platformKey, exeBase string) string {
	exeBase = strings.TrimSuffix(strings.ToLower(exeBase), ".exe") + ".png"
	return "/img/shortcuts/" + SafeFolderName(platformKey) + "/" + exeBase
}

// EnsureCached extracts the exe icon to wwwroot/img/shortcuts/<platform>/<exe>.png if missing or stale.
// wwwroot is the absolute path to the app data wwwroot directory (e.g. .../TcNo Account Switcher/wwwroot).
func EnsureCached(platformKey, exeFullPath, wwwroot string) (publicURL string, err error) {
	exeFullPath = filepath.Clean(exeFullPath)
	base := filepath.Base(exeFullPath)
	www := filepath.Clean(wwwroot)
	dir := filepath.Join(www, "img", "shortcuts", SafeFolderName(platformKey))
	out := filepath.Join(dir, strings.TrimSuffix(strings.ToLower(base), ".exe")+".png")

	if st, err := os.Stat(out); err == nil && !st.IsDir() {
		if exeSt, err := os.Stat(exeFullPath); err == nil && !exeSt.ModTime().After(st.ModTime()) {
			return PublicURL(platformKey, base), nil
		}
	}

	if err := winutil.ExtractExeIcon(exeFullPath, out); err != nil {
		return "", err
	}
	return PublicURL(platformKey, base), nil
}
