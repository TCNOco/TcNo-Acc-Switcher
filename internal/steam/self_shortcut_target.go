package steam

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

// SelfShortcutAppName is what the entry is called in Steam. It is also half of
// how a later run recognises its own entry, so changing it strands the one
// already in a user's library.
const SelfShortcutAppName = "TcNo Account Switcher"

// selfBinaryPrefix matches every name the app ships under: TcNo-Acc-Switcher.exe
// on Windows, tcno-acc-switcher from the deb/rpm/AUR packages, and
// tcno-acc-switcher-<arch>.AppImage.
const selfBinaryPrefix = "tcno-acc-switcher"

// flatpakSteamMarker identifies a Steam root belonging to the Flatpak build. Its
// sandbox has no host /usr, so a host path in Exe launches nothing.
const flatpakSteamMarker = "com.valvesoftware.Steam"

// appImageIconName is the copy taken out of an AppImage mount, kept in the data
// folder because everything under $APPDIR goes away when the app exits.
const appImageIconName = "steam-shortcut-icon.png"

// flatpakSpawn is the only way out of the Flatpak sandbox. It is present in every
// Flatpak runtime, but only works when the Steam Flatpak has been granted
// org.freedesktop.Flatpak talk permission, which it does not ship with.
const flatpakSpawn = "/usr/bin/flatpak-spawn"

// shortcutTarget is one shortcut's launch fields, in the form they are stored.
// Exe and StartDir carry their quote characters as literal bytes, because that
// is how Steam writes them and the quotes are inside the hash the appid comes
// from. Icon and LaunchOptions are unquoted.
type shortcutTarget struct {
	Exe           string
	StartDir      string
	LaunchOptions string
	Icon          string
	// Binary is the unquoted path to the app itself, which is what identifies our
	// entry later. For the Flatpak form it is not what Exe holds.
	Binary string
	// Flatpak reports that this target goes through flatpak-spawn, so the caller
	// can tell the user about the permission it needs.
	Flatpak bool
}

// selfShortcutTargetFor builds the launch fields for one Steam root. goos names
// the target platform rather than the host, and env and exists are injected, so
// every OS and install method can be checked from one table.
func selfShortcutTargetFor(goos, exePath, steamRoot string, env func(string) string, exists func(string) bool) shortcutTarget {
	binary := resolveSelfBinaryPath(goos, exePath, env, exists)
	dir := pathDir(goos, binary)

	target := shortcutTarget{
		Exe:      quotePath(binary),
		StartDir: quotePath(withTrailingSeparator(goos, dir)),
		Icon:     probeSelfIcon(goos, binary, exists),
		Binary:   binary,
	}

	if goos != "windows" && strings.Contains(steamRoot, flatpakSteamMarker) {
		target.Exe = quotePath(flatpakSpawn)
		target.StartDir = quotePath(withTrailingSeparator(goos, pathDir(goos, flatpakSpawn)))
		target.LaunchOptions = "--host " + quotePath(binary)
		target.Flatpak = true
	}
	return target
}

// resolveSelfShortcutTarget builds the launch fields for the running app.
func resolveSelfShortcutTarget(steamRoot string) (shortcutTarget, error) {
	exe, err := os.Executable()
	if err != nil {
		return shortcutTarget{}, err
	}
	target := selfShortcutTargetFor(runtime.GOOS, exe, steamRoot, os.Getenv, fileExists)
	if target.Icon == "" {
		target.Icon = stableAppImageIcon()
	}
	return target, nil
}

// resolveSelfBinaryPath answers what a shortcut should point at, which is not
// always os.Executable(). Inside an AppImage that is a path under an ephemeral
// /tmp mount that is gone once the app exits, so $APPIMAGE is preferred.
func resolveSelfBinaryPath(goos, exePath string, env func(string) string, exists func(string) bool) string {
	if goos != "windows" {
		if appImage := strings.TrimSpace(env("APPIMAGE")); appImage != "" && exists(appImage) {
			return appImage
		}
	}
	return exePath
}

// probeSelfIcon returns a path Steam can draw the tile from, or "" to let it fall
// back to a plain one.
func probeSelfIcon(goos, binary string, exists func(string) bool) string {
	switch goos {
	case "windows":
		// The icons are embedded in the binary rather than shipped beside it, so
		// the exe is the icon.
		return binary
	case "darwin":
		// .../TcNo-Acc-Switcher.app/Contents/MacOS/<binary>
		contents := pathDir(goos, pathDir(goos, binary))
		if icns := joinPath(goos, contents, "Resources", "icons.icns"); exists(icns) {
			return icns
		}
		return ""
	default:
		// The packages install into hicolor under the prefix they were installed
		// with, so derive that from the binary before falling back to the two
		// standard ones.
		prefix := pathDir(goos, pathDir(goos, binary))
		for _, root := range []string{prefix, "/usr", "/usr/local"} {
			candidate := joinPath(goos, root, "share", "icons", "hicolor", "256x256", "apps", selfBinaryPrefix+".png")
			if exists(candidate) {
				return candidate
			}
		}
		return ""
	}
}

// stableAppImageIcon copies the icon out of the AppImage's mount into the data
// folder and returns that copy. Everything under $APPDIR disappears when the app
// exits, so pointing Steam straight at it would leave a blank tile.
func stableAppImageIcon() string {
	appDir := strings.TrimSpace(os.Getenv("APPDIR"))
	if appDir == "" || strings.TrimSpace(os.Getenv("APPIMAGE")) == "" {
		return ""
	}
	dataRoot, err := paths.DataRoot()
	if err != nil {
		return ""
	}
	dest := filepath.Join(dataRoot, appImageIconName)
	// Reading the state of the row must not keep rewriting this.
	if fileExists(dest) {
		return dest
	}
	for _, name := range []string{selfBinaryPrefix + ".png", ".DirIcon"} {
		data, err := os.ReadFile(filepath.Join(appDir, name))
		if err != nil {
			continue
		}
		if err := fsutil.WriteFileAtomic(dest, data, 0o644); err != nil {
			return ""
		}
		return dest
	}
	return ""
}

// isSelfBinaryPath reports whether a path names this app, under any of the names
// its install methods give it.
func isSelfBinaryPath(p string) bool {
	base := strings.ToLower(strings.Trim(strings.TrimSpace(p), `"`))
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	return strings.HasPrefix(base, selfBinaryPrefix)
}

func quotePath(p string) string { return `"` + p + `"` }

func pathSeparator(goos string) string {
	if goos == "windows" {
		return `\`
	}
	return "/"
}

// pathDir is filepath.Dir for a named platform rather than the host one.
func pathDir(goos, p string) string {
	sep := pathSeparator(goos)
	i := strings.LastIndexAny(p, `/\`)
	if goos != "windows" {
		i = strings.LastIndex(p, "/")
	}
	if i < 0 {
		return "."
	}
	if i == 0 {
		return sep
	}
	return p[:i]
}

func joinPath(goos string, parts ...string) string {
	sep := pathSeparator(goos)
	out := parts[0]
	for _, part := range parts[1:] {
		out = strings.TrimSuffix(out, sep) + sep + part
	}
	return out
}

// withTrailingSeparator matches how Steam writes StartDir, which keeps the
// separator inside the quotes.
func withTrailingSeparator(goos, dir string) string {
	sep := pathSeparator(goos)
	if strings.HasSuffix(dir, sep) {
		return dir
	}
	return dir + sep
}
