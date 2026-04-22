package shortcuts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/winutil"

)

// CreateAccountShortcut writes a Desktop .lnk targeting this exe with CLI swap args.
func CreateAccountShortcut(platformKey, uniqueID, displayName, stateSuffix string) (string, error) {
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
		title = fmt.Sprintf("%s [%s]", title, strings.TrimSpace(stateSuffix))
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
		icon = p + ",0"
	}

	if err := winutil.WriteShortcutLnk(outPath, self, argv, icon); err != nil {
		return "", err
	}
	return outPath, nil
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
