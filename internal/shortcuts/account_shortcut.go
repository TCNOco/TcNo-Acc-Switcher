package shortcuts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/winutil"
)

// CreateAccountShortcut builds a Desktop .lnk; for Steam, stateSuffix is persona argv index and stateTitle is an optional filename segment.
func CreateAccountShortcut(platformKey, uniqueID, displayName, stateSuffix, stateTitle, accountLogin string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	if platformKey == "" || uniqueID == "" {
		return "", fmt.Errorf("missing platform or account id")
	}

	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	self = filepath.Clean(self)

	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	if desktop == "" {
		return "", fmt.Errorf("desktop path unknown")
	}

	title := accountShortcutDesktopBaseName(platformKey, uniqueID, displayName, stateSuffix, stateTitle, accountLogin)

	outPath := filepath.Join(desktop, title+".lnk")

	var argv string
	if strings.EqualFold(platformKey, "Steam") {
		argv = "+s:" + uniqueID
		if strings.TrimSpace(stateSuffix) != "" {
			argv += ":" + strings.TrimSpace(stateSuffix)
		}
	} else {
		idx, err := cli.LoadPlatformIndex()
		if err != nil {
			return "", err
		}
		short := cli.ShortTokenForPlatform(idx, platformKey)
		if short == "" {
			return "", fmt.Errorf("platform %q has no CLI short token", platformKey)
		}
		argv = "+" + short + ":" + uniqueID
	}

	icon := ""
	if p, ok := profileimage.CachedFilePath(platformKey, uniqueID); ok && p != "" {
		if root, err := paths.DataRoot(); err == nil {
			cacheDir := filepath.Join(root, "IconCache")
			if err := os.MkdirAll(cacheDir, 0o755); err == nil {
				platformIcoPath := filepath.Join(cacheDir, profileimage.PlatformFolder(platformKey)+"_platform.ico")
				if _, err := os.Stat(platformIcoPath); err != nil {
					_ = winutil.BuildPlatformIcon(platformKey, platformIcoPath)
				}
				icoName := fmt.Sprintf("%s_%s.ico", profileimage.PlatformFolder(platformKey), sanitizeShortcutFileName(uniqueID))
				icoPath := filepath.Join(cacheDir, icoName)
				if err := winutil.BuildCombinedAccountIcon(platformKey, p, icoPath, platformIcoPath); err == nil {
					icon = icoPath + ",0"
				}
			}
		}
		if icon == "" {
			icon = p + ",0"
		}
	}

	workDir := filepath.Dir(self)
	desc := fmt.Sprintf("TcNo Account Switcher - %s - %s", platformKey, title)
	if err := winutil.WriteShortcutLnk(outPath, self, argv, workDir, desc, icon); err != nil {
		return "", err
	}
	return outPath, nil
}

func shortcutFilenameFallback(platformKey, uniqueID, accountLogin string) string {
	_ = platformKey
	login := strings.TrimSpace(accountLogin)
	if login != "" {
		if s := shellShortcutStem(login); s != "" && !isDegenerateShortcutBasename(s) {
			return sanitizeShortcutFileName(s)
		}
	}
	if s := shellShortcutStem(uniqueID); s != "" && !isDegenerateShortcutBasename(s) {
		return sanitizeShortcutFileName(s)
	}
	return "TcNoShortcut"
}

func shellShortcutStem(s string) string {
	return strings.TrimSpace(paths.ShellShortcutBaseName(strings.TrimSpace(s), 180))
}

func resolvedAccountShortcutStem(platformKey, uniqueID, displayName, accountLogin string) string {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	base := strings.TrimSpace(displayName)
	if base == "" {
		base = uniqueID
	}
	sanBase := shellShortcutStem(base)
	if sanBase == "" || isDegenerateShortcutBasename(sanBase) {
		return shortcutFilenameFallback(platformKey, uniqueID, accountLogin)
	}
	return sanitizeShortcutFileName(sanBase)
}

func ResolvedAccountShortcutStem(platformKey, uniqueID, displayName, accountLogin string) string {
	return resolvedAccountShortcutStem(platformKey, uniqueID, displayName, accountLogin)
}

func accountShortcutDesktopBaseName(platformKey, uniqueID, displayName, stateSuffix, stateTitle, accountLogin string) string {
	var stateLabel string
	steamState := strings.EqualFold(platformKey, "Steam") && strings.TrimSpace(stateSuffix) != ""
	if steamState {
		stateLabel = strings.TrimSpace(stateTitle)
		if stateLabel == "" {
			stateLabel = steamPersonaStateFileLabel(strings.TrimSpace(stateSuffix))
		}
	}

	sanBase := resolvedAccountShortcutStem(platformKey, uniqueID, displayName, accountLogin)
	if steamState && stateLabel != "" {
		return sanitizeShortcutFileName(fmt.Sprintf("%s (%s)", sanBase, stateLabel))
	}
	return sanBase
}

func isDegenerateShortcutBasename(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func steamPersonaStateFileLabel(numeric string) string {
	switch strings.TrimSpace(numeric) {
	case "0":
		return "Offline"
	case "1":
		return "Online"
	case "2":
		return "Busy"
	case "3":
		return "Away"
	case "4":
		return "Snooze"
	case "5":
		return "Looking to trade"
	case "6":
		return "Looking to play"
	case "7":
		return "Invisible"
	default:
		return numeric
	}
}

func sanitizeShortcutFileName(name string) string {
	out := paths.ShellShortcutBaseName(name, 180)
	if out == "" {
		return "TcNoShortcut"
	}
	return out
}
