package shortcuts

import (
	"os"
	"strings"

	"TcNo-Acc-Switcher/internal/winutil"
)

// ShortcutResult reports where a shortcut ended up and whether it was already
// there, so the UI can say it already exists rather than claiming to have made
// one that was sitting on the desktop the whole time.
type ShortcutResult struct {
	Path           string `json:"path"`
	AlreadyExisted bool   `json:"alreadyExisted"`
}

// shortcutAlreadyMatches reports whether path already holds a shortcut that
// launches exactly this and, when requested, uses the generated icon. This lets
// recreating a shortcut repair an empty or outdated icon reference.
func shortcutAlreadyMatches(path, target, arguments, iconLocation string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	gotTarget, gotArgs, gotIcon, err := winutil.ReadLnkShortcut(path)
	if err != nil {
		return false
	}
	// ReadLnkShortcut returns the icon's path string without the header's index;
	// all generated switcher ICO references use index zero.
	if iconLocation != "" && !strings.EqualFold(strings.TrimSpace(gotIcon), strings.TrimSuffix(iconLocation, ",0")) {
		return false
	}
	// The target is a path, so case-insensitive; the arguments are ours and are
	// compared exactly, so a difference there rewrites rather than silently
	// reporting a shortcut that points somewhere else.
	return strings.EqualFold(strings.TrimSpace(gotTarget), strings.TrimSpace(target)) &&
		strings.TrimSpace(gotArgs) == strings.TrimSpace(arguments)
}
