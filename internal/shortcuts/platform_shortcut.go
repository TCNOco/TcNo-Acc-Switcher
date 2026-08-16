package shortcuts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/winutil"
)

// platformSwitcherLnkName returns the .lnk basename for the "open this platform in TcNo" shortcut.
func platformSwitcherLnkName(platformKey string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return "", fmt.Errorf("missing platform")
	}
	return "TcNo - " + sanitizeShortcutFileName(platformKey) + " Switcher.lnk", nil
}

// PlatformShortcutExists reports whether the platform switcher .lnk exists on any of the user's
// Desktop folders — a shortcut written before or after a Known Folder Move still counts.
func PlatformShortcutExists(platformKey string) (bool, error) {
	name, err := platformSwitcherLnkName(platformKey)
	if err != nil {
		return false, err
	}
	for _, dir := range winutil.DesktopSearchDirs() {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

// CreatePlatformShortcut writes a Desktop .lnk targeting this exe; arguments open the platform page in the app.
func CreatePlatformShortcut(platformKey string) (ShortcutResult, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return ShortcutResult{}, fmt.Errorf("missing platform")
	}

	self, err := os.Executable()
	if err != nil {
		return ShortcutResult{}, err
	}
	self = filepath.Clean(self)

	name, err := platformSwitcherLnkName(platformKey)
	if err != nil {
		return ShortcutResult{}, err
	}
	desktop, err := winutil.DesktopWriteDir()
	if err != nil {
		return ShortcutResult{}, err
	}
	outPath := filepath.Join(desktop, name)

	icon := ""
	if root, err := paths.DataRoot(); err == nil {
		cacheDir := filepath.Join(root, "IconCache")
		if err := os.MkdirAll(cacheDir, 0o755); err == nil {
			icoName := profileimage.PlatformFolder(platformKey) + "_platform.ico"
			icoPath := filepath.Join(cacheDir, icoName)
			if err := winutil.BuildPlatformIcon(platformKey, icoPath); err == nil {
				icon = icoPath + ",0"
			}
		}
	}

	workDir := filepath.Dir(self)
	desc := fmt.Sprintf("TcNo Account Switcher - %s", platformKey)
	argv := platformKey
	appID := winutil.ShortcutAppUserModelID("platform", platformKey)
	if shortcutAlreadyMatches(outPath, self, argv) {
		return ShortcutResult{Path: outPath, AlreadyExisted: true}, nil
	}
	if err := winutil.WriteShortcutLnk(outPath, self, argv, workDir, desc, icon, appID); err != nil {
		return ShortcutResult{}, err
	}
	return ShortcutResult{Path: outPath}, nil
}

// DeletePlatformShortcut removes the platform .lnk from every Desktop folder it may have landed in.
func DeletePlatformShortcut(platformKey string) error {
	name, err := platformSwitcherLnkName(platformKey)
	if err != nil {
		return err
	}
	for _, dir := range winutil.DesktopSearchDirs() {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
