package shortcuts

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/winutil"
)

// Steam can cache only <id>_static, or a video at <id> alongside that still.
// Shell icons need a decodable image. A manual avatar must never fall back to
// the account's old Steam avatar.
func cachedShortcutAvatar(platformKey, uniqueID string) (string, bool) {
	ids := []string{uniqueID}
	if strings.EqualFold(platformKey, "Steam") && !profileimage.HasManualProfileMarker(platformKey, uniqueID) {
		ids = append(ids, uniqueID+"_static")
	}
	for _, id := range ids {
		p, ok := profileimage.CachedFilePath(platformKey, id)
		if !ok {
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		_, _, err = image.DecodeConfig(f)
		_ = f.Close()
		if err == nil {
			return p, true
		}
	}
	return "", false
}

func buildAccountShortcutIcon(platformKey, uniqueID string) string {
	root, err := paths.DataRoot()
	if err != nil {
		return ""
	}
	cacheDir := filepath.Join(root, "IconCache")
	platformICO := filepath.Join(cacheDir, profileimage.PlatformFolder(platformKey)+"_platform.ico")
	if st, err := os.Stat(platformICO); err != nil || st.IsDir() || st.Size() == 0 {
		if err := winutil.BuildPlatformIcon(platformKey, platformICO); err != nil {
			slog.Warn("shortcut platform icon unavailable", "platform", platformKey, "err", err)
		}
	}
	if avatar, ok := cachedShortcutAvatar(platformKey, uniqueID); ok {
		name := fmt.Sprintf("%s_%s.ico", profileimage.PlatformFolder(platformKey), sanitizeShortcutFileName(uniqueID))
		ico := filepath.Join(cacheDir, name)
		if err := winutil.BuildCombinedAccountIcon(platformKey, avatar, ico, platformICO); err == nil {
			return ico + ",0"
		} else {
			slog.Warn("shortcut account icon unavailable", "platform", platformKey, "err", err)
		}
	}
	// PNG/JPEG/video paths are not shell icon resources. Use the platform ICO
	// when no avatar is usable, leaving the executable as the final fallback.
	if st, err := os.Stat(platformICO); err == nil && !st.IsDir() && st.Size() > 0 {
		return platformICO + ",0"
	}
	return ""
}
