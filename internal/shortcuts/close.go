package shortcuts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// targetExeCache remembers what a .lnk points at, keyed on its path and the
// modification time and size it had when read. Resolving one goes through the
// shell, and buildDTOs resolves every shortcut a platform has, several times
// over a page open; a stat is what it costs to confirm the answer still holds.
var targetExeCache sync.Map // string -> targetExeEntry

type targetExeEntry struct {
	modTime time.Time
	size    int64
	exe     string
}

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

	st, statErr := os.Stat(full)
	if statErr == nil {
		if cached, ok := targetExeCache.Load(full); ok {
			if e := cached.(targetExeEntry); e.modTime.Equal(st.ModTime()) && e.size == st.Size() {
				return e.exe
			}
		}
	}

	target, _, _, err := readLnkShortcut(full)
	if err != nil {
		return ""
	}
	base := filepath.Base(strings.TrimSpace(target))
	if !strings.HasSuffix(strings.ToLower(base), ".exe") {
		base = ""
	}
	if statErr == nil {
		targetExeCache.Store(full, targetExeEntry{modTime: st.ModTime(), size: st.Size(), exe: base})
	}
	return base
}

// readLnkShortcut is winutil.ReadLnkShortcut behind a variable so tests can
// count resolves without needing the shell and real .lnk files.
var readLnkShortcut = winutil.ReadLnkShortcut

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
