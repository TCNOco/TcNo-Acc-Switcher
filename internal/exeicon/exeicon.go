package exeicon

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"TcNo-Acc-Switcher/internal/winutil"
)

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

	if cachedIconIsFresh(out, exeFullPath) {
		return PublicURL(platformKey, base), nil
	}

	if err := extractExeIcon(exeFullPath, out); err != nil {
		return "", err
	}
	return PublicURL(platformKey, base), nil
}

// cachedIconIsFresh reports whether the cached PNG exists and is no older than
// the file it was extracted from.
func cachedIconIsFresh(out, source string) bool {
	st, err := os.Stat(out)
	if err != nil || st.IsDir() {
		return false
	}
	srcSt, err := os.Stat(source)
	if err != nil {
		return false
	}
	return !srcSt.ModTime().After(st.ModTime())
}

func EnsureShortcutCached(platformKey, exeBase, shortcutPath, wwwroot string) (publicURL string, err error) {
	exeBase = strings.TrimSpace(exeBase)
	shortcutPath = filepath.Clean(strings.TrimSpace(shortcutPath))
	if exeBase == "" || shortcutPath == "" {
		return "", os.ErrInvalid
	}
	www := filepath.Clean(wwwroot)
	dir := filepath.Join(www, "img", "shortcuts", SafeFolderName(platformKey))
	out := filepath.Join(dir, strings.TrimSuffix(strings.ToLower(exeBase), ".exe")+".png")

	// Same guard as EnsureCached: resolving the shortcut through the shell and
	// re-encoding the PNG is too expensive to repeat on every page open.
	if cachedIconIsFresh(out, shortcutPath) {
		return PublicURL(platformKey, exeBase), nil
	}

	if err := extractShortcutIcon(shortcutPath, out); err != nil {
		return "", err
	}
	return PublicURL(platformKey, exeBase), nil
}

// Extraction goes through these so tests can count calls without needing a real
// shortcut and the shell to resolve it.
var (
	extractExeIcon      = winutil.ExtractExeIcon
	extractShortcutIcon = winutil.ExtractShortcutIcon
)
