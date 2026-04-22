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

// CreateAccountShortcut writes a Desktop .lnk targeting this exe with CLI swap args.
// For Steam, stateSuffix is the persona state index for argv; stateTitle is shown in the filename (optional, localized).
func CreateAccountShortcut(platformKey, uniqueID, displayName, stateSuffix, stateTitle string) (string, error) {
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

	title := strings.TrimSpace(displayName)
	if title == "" {
		title = uniqueID
	}
	title = sanitizeShortcutFileName(title)
	if strings.EqualFold(platformKey, "Steam") && strings.TrimSpace(stateSuffix) != "" {
		label := strings.TrimSpace(stateTitle)
		if label == "" {
			label = steamPersonaStateFileLabel(strings.TrimSpace(stateSuffix))
		}
		label = sanitizeShortcutFileName(label)
		if label != "" {
			title = fmt.Sprintf("%s (%s)", title, label)
		}
	}

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
				icoName := fmt.Sprintf("%s_%s.ico", profileimage.PlatformFolder(platformKey), sanitizeShortcutFileName(uniqueID))
				icoPath := filepath.Join(cacheDir, icoName)
				if err := winutil.BuildCombinedAccountIcon(platformKey, p, icoPath); err == nil {
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
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return "TcNoShortcut"
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}
