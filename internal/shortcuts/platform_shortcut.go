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

// platformSwitcherLnkPath returns the Desktop path for the "open this platform in TcNo" shortcut.
func platformSwitcherLnkPath(platformKey string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return "", fmt.Errorf("missing platform")
	}
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	if desktop == "" || strings.TrimSpace(os.Getenv("USERPROFILE")) == "" {
		return "", fmt.Errorf("desktop path unknown")
	}
	base := "TcNo - " + sanitizeShortcutFileName(platformKey) + " Switcher"
	return filepath.Join(desktop, base+".lnk"), nil
}

// PlatformShortcutExists reports whether the platform switcher .lnk exists on the user's Desktop.
func PlatformShortcutExists(platformKey string) (bool, error) {
	p, err := platformSwitcherLnkPath(platformKey)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreatePlatformShortcut writes a Desktop .lnk targeting this exe; arguments open the platform page in the app.
func CreatePlatformShortcut(platformKey string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return "", fmt.Errorf("missing platform")
	}

	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	self = filepath.Clean(self)

	outPath, err := platformSwitcherLnkPath(platformKey)
	if err != nil {
		return "", err
	}

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
	if err := winutil.WriteShortcutLnk(outPath, self, argv, workDir, desc, icon, appID); err != nil {
		return "", err
	}
	return outPath, nil
}

// DeletePlatformShortcut removes the Desktop .lnk for this platform if it exists.
func DeletePlatformShortcut(platformKey string) error {
	p, err := platformSwitcherLnkPath(platformKey)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
