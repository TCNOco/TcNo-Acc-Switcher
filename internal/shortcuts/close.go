package shortcuts

import (
	"fmt"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// shortcutTargetExe returns the executable a shortcut starts, or "" when it
// starts something that is not a program. A .url opens a web or protocol
// address and a .lnk may point at a folder or a document, and none of those
// leave a process to close.
func shortcutTargetExe(platformKey, fileName string) string {
	if !strings.HasSuffix(strings.ToLower(fileName), ".lnk") {
		return ""
	}
	full, err := resolveShortcutPath(platformKey, fileName)
	if err != nil {
		return ""
	}
	target, _, _, err := winutil.ReadLnkShortcut(full)
	if err != nil {
		return ""
	}
	base := filepath.Base(strings.TrimSpace(target))
	if !strings.HasSuffix(strings.ToLower(base), ".exe") {
		return ""
	}
	return base
}

// CloseShortcut ends the program a shortcut started, using the platform's own
// closing method so exiting a game behaves like exiting its platform.
func (s *Service) CloseShortcut(platformKey, fileName string) error {
	platformKey = strings.TrimSpace(platformKey)
	fileName = filepath.Base(strings.TrimSpace(fileName))
	if fileName == "" || fileName == "." || fileName == ".." {
		return fmt.Errorf("invalid shortcut name")
	}
	exe := shortcutTargetExe(platformKey, fileName)
	if exe == "" {
		return fmt.Errorf("shortcut does not start a program")
	}
	method := winutil.ClosingCombined
	if ps, err := platform.LoadPlatformSettings(platformKey); err == nil {
		method = winutil.ClosingMethod(platform.NormalizeClosingMethod(ps.ClosingMethod))
	}
	names := []string{exe}
	if err := winutil.ErrIfCannotKill(names, method); err != nil {
		return err
	}
	return winutil.KillByName(names, method, nil)
}
