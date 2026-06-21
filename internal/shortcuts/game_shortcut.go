package shortcuts

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/winutil"
)

var steamRungameIDRE = regexp.MustCompile(`(?i)steam://rungameid/(\d+)`)

// CreateGameAccountShortcut: steam://rungameid .url tiles use --run-appid; otherwise --run-shortcut.
func CreateGameAccountShortcut(platformKey, uniqueID, accountDisplayName, accountLogin, gameFileName string) (string, error) {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	gameFileName = filepath.Base(strings.TrimSpace(gameFileName))
	if platformKey == "" || uniqueID == "" {
		return "", fmt.Errorf("missing platform or account id")
	}
	if gameFileName == "" || !isShortcutFile(gameFileName) {
		return "", fmt.Errorf("invalid game shortcut name")
	}

	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return "", err
	}
	cacheFull := filepath.Join(root, "Shortcuts", gameFileName)
	if st, err := os.Stat(cacheFull); err != nil || st.IsDir() {
		return "", fmt.Errorf("game shortcut not found in cache")
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

	title := gameAccountShortcutDesktopBaseName(platformKey, uniqueID, accountDisplayName, accountLogin, gameFileName)
	outPath := filepath.Join(desktop, title+".lnk")

	var argv string
	if strings.EqualFold(platformKey, "Steam") {
		argv = "+s:" + uniqueID
		if appID, ok := steamRungameIDFromURLFile(cacheFull); ok {
			argv += " --run-appid=" + appID
		} else {
			argv += " --run-shortcut=" + url.QueryEscape(gameFileName)
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
		argv = "+" + short + ":" + uniqueID + " --run-shortcut=" + url.QueryEscape(gameFileName)
	}

	icon := buildGameAccountShortcutIcon(platformKey, uniqueID, gameFileName)

	workDir := filepath.Dir(self)
	desc := fmt.Sprintf("TcNo Account Switcher - %s - %s", platformKey, title)
	appID := winutil.ShortcutAppUserModelID(platformKey, uniqueID, gameFileName)
	if err := winutil.WriteShortcutLnk(outPath, self, argv, workDir, desc, icon, appID); err != nil {
		return "", err
	}
	return outPath, nil
}

func steamRungameIDFromURLFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	m := steamRungameIDRE.FindSubmatch(b)
	if len(m) < 2 {
		return "", false
	}
	return string(m[1]), true
}

func gameAccountShortcutDesktopBaseName(platformKey, uniqueID, displayName, accountLogin, gameFileName string) string {
	gameStem := removeShortcutExt(gameFileName)
	sanGame := sanitizeShortcutFileName(gameStem)
	if isDegenerateShortcutBasename(sanGame) {
		sanGame = "Game"
	}

	sanAcc := resolvedAccountShortcutStem(platformKey, uniqueID, displayName, accountLogin)
	return sanitizeShortcutFileName(fmt.Sprintf("%s (%s)", sanGame, sanAcc))
}

func buildGameAccountShortcutIcon(platformKey, uniqueID, gameFileName string) string {
	p, ok := profileimage.CachedFilePath(platformKey, uniqueID)
	if !ok || p == "" {
		return ""
	}
	root, err := paths.DataRoot()
	if err != nil {
		return fallbackAvatarIconOnly(p)
	}
	cacheDir := filepath.Join(root, "IconCache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fallbackAvatarIconOnly(p)
	}

	gameIconPath, err := iconDiskPath(platformKey, gameFileName)
	if err == nil && gameIconPath != "" {
		if _, err := os.Stat(gameIconPath); err == nil {
			icoName := fmt.Sprintf("%s_game_%s_%s.ico",
				profileimage.PlatformFolder(platformKey),
				sanitizeShortcutFileName(removeShortcutExt(gameFileName)),
				sanitizeShortcutFileName(uniqueID))
			icoPath := filepath.Join(cacheDir, icoName)
			if err := winutil.BuildCombinedGameIcon(gameIconPath, p, icoPath); err == nil {
				return icoPath + ",0"
			}
		}
	}

	platformIcoPath := filepath.Join(cacheDir, profileimage.PlatformFolder(platformKey)+"_platform.ico")
	if _, err := os.Stat(platformIcoPath); err != nil {
		_ = winutil.BuildPlatformIcon(platformKey, platformIcoPath)
	}
	icoName := fmt.Sprintf("%s_%s.ico", profileimage.PlatformFolder(platformKey), sanitizeShortcutFileName(uniqueID))
	icoPath := filepath.Join(cacheDir, icoName)
	if err := winutil.BuildCombinedAccountIcon(platformKey, p, icoPath, platformIcoPath); err == nil {
		return icoPath + ",0"
	}
	return fallbackAvatarIconOnly(p)
}

func fallbackAvatarIconOnly(p string) string {
	if p != "" {
		return p + ",0"
	}
	return ""
}
