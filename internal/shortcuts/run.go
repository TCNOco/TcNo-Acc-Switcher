package shortcuts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/exeicon"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// RunShortcut launches a cached shortcut from LoginCache/<platform>/Shortcuts/.
func RunShortcut(platformKey, fileName string, admin bool) error {
	platformKey = strings.TrimSpace(platformKey)
	fileName = filepath.Base(strings.TrimSpace(fileName))
	if fileName == "" || fileName == "." || fileName == ".." {
		return fmt.Errorf("invalid shortcut name")
	}
	if !isShortcutFile(fileName) {
		return fmt.Errorf("not a shortcut file")
	}
	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return err
	}
	full := filepath.Join(root, "Shortcuts", fileName)
	if st, err := os.Stat(full); err != nil || st.IsDir() {
		return fmt.Errorf("shortcut not found")
	}

	low := strings.ToLower(fileName)
	if strings.HasSuffix(low, ".url") {
		return winutil.Start("cmd.exe", []string{"/C", full}, winutil.StartOpts{HideWindow: true})
	}

	// .lnk
	if admin {
		return winutil.Start(full, nil, winutil.StartOpts{
			Admin:         true,
			AsDesktopUser: winutil.IsProcessElevated() && !admin,
		})
	}
	if winutil.IsProcessElevated() {
		return winutil.Start("explorer.exe", []string{full}, winutil.StartOpts{})
	}
	return winutil.Start("cmd.exe", []string{"/C", "start", "", full}, winutil.StartOpts{})
}

// HideShortcut renames the cached shortcut to *_ignored.* and removes it from settings.
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
