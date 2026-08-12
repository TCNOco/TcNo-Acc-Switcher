package shortcuts

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/steam"
)

var reSteamRunGameID = regexp.MustCompile(`(?i)^\s*url\s*=\s*steam://rungameid/(\d+)`)

// steamShortcutAppID reads the app id out of a Steam Start Menu .url shortcut.
//
// rungameid packs mod and non-Steam entries into a 64-bit value whose top 32
// bits are the app id; an ordinary game is just the id on its own.
func steamShortcutAppID(urlPath string) string {
	if !strings.EqualFold(filepath.Ext(urlPath), ".url") {
		return ""
	}
	data, err := os.ReadFile(urlPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		m := reSteamRunGameID.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil || id == 0 {
			return ""
		}
		if id > 0xFFFFFFFF {
			id >>= 32
		}
		return strconv.FormatUint(id, 10)
	}
	return ""
}

// ensureSteamShortcutIcon writes outPNG from Steam's own artwork cache for the
// game a .url shortcut launches.
//
// Steam only downloads a game's <hash>.ico into steam\games when it has drawn a
// Start Menu shortcut for it, and prunes them; Counter-Strike 2 is a standing
// example of a shortcut whose IconFile points at a file that is not there. The
// icon extractor has nothing to read in that case and writes a 1x1 placeholder,
// so the librarycache art the games view already caches is used instead.
//
// Disk only - a scan must not block on the network. Reports whether outPNG was
// written.
func ensureSteamShortcutIcon(platformKey, shortcutPath, outPNG, www string) bool {
	if !strings.EqualFold(platformKey, "Steam") {
		return false
	}
	appID := steamShortcutAppID(shortcutPath)
	if appID == "" {
		return false
	}
	url, err := steam.EnsureLocalGameIcon(appID)
	if err != nil || url == "" {
		return false
	}
	src := filepath.Join(www, filepath.FromSlash(strings.TrimPrefix(url, "/")))
	data, err := os.ReadFile(src)
	if err != nil {
		return false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return false
	}
	return fsutil.WriteFileAtomic(outPNG, buf.Bytes(), 0o644) == nil
}
