package shortcuts

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/exeicon"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/winutil"
)

// RunShortcut uses LoginCache copy, else Desktop (non-recursive), else Start Menu (recursive).
func RunShortcut(platformKey, fileName string, admin bool) error {
	platformKey = strings.TrimSpace(platformKey)
	fileName = filepath.Base(strings.TrimSpace(fileName))
	if fileName == "" || fileName == "." || fileName == ".." {
		return fmt.Errorf("invalid shortcut name")
	}
	if !isShortcutFile(fileName) {
		return fmt.Errorf("not a shortcut file")
	}
	full, err := resolveShortcutPath(platformKey, fileName)
	if err != nil {
		return err
	}

	low := strings.ToLower(filepath.Base(full))
	var startErr error
	if strings.HasSuffix(low, ".url") {
		startErr = winutil.Start("cmd.exe", []string{"/C", full}, winutil.StartOpts{HideWindow: true})
	} else if admin {
		startErr = winutil.Start(full, nil, winutil.StartOpts{
			Admin:         true,
			AsDesktopUser: winutil.IsProcessElevated() && !admin,
		})
	} else if winutil.IsProcessElevated() {
		startErr = winutil.Start("explorer.exe", []string{full}, winutil.StartOpts{})
	} else {
		startErr = winutil.Start("cmd.exe", []string{"/C", "start", "", full}, winutil.StartOpts{})
	}
	if startErr == nil {
		_ = stats.IncrementGamesLaunched(platformKey)
	}
	return startErr
}

func shortcutNameIgnored(name string) bool {
	return strings.Contains(strings.ToLower(name), "_ignored")
}

// matchesShortcutRequest matches basename or same stem across .lnk/.url.
func matchesShortcutRequest(want, candidate string) bool {
	if shortcutNameIgnored(candidate) {
		return false
	}
	if !isShortcutFile(candidate) {
		return false
	}
	if strings.EqualFold(want, candidate) {
		return true
	}
	return strings.EqualFold(removeShortcutExt(want), removeShortcutExt(candidate))
}

func resolveShortcutPath(platformKey, fileName string) (string, error) {
	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return "", err
	}
	cacheFull := filepath.Join(root, "Shortcuts", fileName)
	if st, err := os.Stat(cacheFull); err == nil && !st.IsDir() {
		return cacheFull, nil
	}

	for _, dir := range desktopRoots() {
		if p := findShortcutInDir(dir, fileName); p != "" {
			return p, nil
		}
	}

	for _, base := range startMenuProgramRoots() {
		var found string
		_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || found != "" {
				if found != "" {
					return fs.SkipAll
				}
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if matchesShortcutRequest(fileName, d.Name()) {
				found = path
				return fs.SkipAll
			}
			return nil
		})
		if found != "" {
			return found, nil
		}
	}

	return "", fmt.Errorf("shortcut not found")
}

func desktopRoots() []string {
	var out []string
	if up := strings.TrimSpace(os.Getenv("USERPROFILE")); up != "" {
		out = append(out, filepath.Join(up, "Desktop"))
	}
	if pub := strings.TrimSpace(os.Getenv("PUBLIC")); pub != "" {
		out = append(out, filepath.Join(pub, "Desktop"))
	}
	return out
}

func startMenuProgramRoots() []string {
	var out []string
	if app := strings.TrimSpace(os.Getenv("APPDATA")); app != "" {
		out = append(out, filepath.Join(app, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if pd := strings.TrimSpace(os.Getenv("ProgramData")); pd != "" {
		out = append(out, filepath.Join(pd, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	return out
}

func findShortcutInDir(dir, fileName string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.EqualFold(fileName, n) && !shortcutNameIgnored(n) && isShortcutFile(n) {
			return filepath.Join(dir, n)
		}
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if matchesShortcutRequest(fileName, n) {
			return filepath.Join(dir, n)
		}
	}
	return ""
}

func HideShortcut(platformKey, fileName string) error {
	platformKey = strings.TrimSpace(platformKey)
	fileName = filepath.Base(strings.TrimSpace(fileName))
	if fileName == "" {
		return fmt.Errorf("invalid shortcut name")
	}
	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "Shortcuts")
	src := filepath.Join(dir, fileName)
	if _, err := os.Stat(src); err != nil {
		return err
	}
	dst := filepath.Join(dir, filepath.Base(ignoredName(fileName)))
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if p, err := iconDiskPath(platformKey, fileName); err == nil && p != "" {
		_ = os.Remove(p)
	}

	entries, err := loadEntries(platformKey)
	if err != nil {
		return err
	}
	out := entries[:0]
	for _, e := range entries {
		if strings.EqualFold(e.FileName, fileName) {
			continue
		}
		out = append(out, e)
	}
	return saveEntries(platformKey, out)
}

func iconDiskPath(platformKey, fileName string) (string, error) {
	www, err := platform.WwwrootDir()
	if err != nil {
		return "", err
	}
	stem := removeShortcutExt(fileName)
	if stem == "" {
		return "", fmt.Errorf("empty stem")
	}
	return filepath.Join(www, "img", "shortcuts", exeicon.SafeFolderName(platformKey), strings.ToLower(stem)+".png"), nil
}
