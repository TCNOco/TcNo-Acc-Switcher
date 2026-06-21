package shortcuts

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/exeicon"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

func steamShortcutSourceDirs() []string {
	app := os.Getenv("APPDATA")
	if app == "" {
		return nil
	}
	return []string{filepath.Join(app, "Microsoft", "Windows", "Start Menu", "Programs", "Steam")}
}

func copyIfNewer(src, dst string) error {
	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	di, err := os.Stat(dst)
	if err == nil && !di.IsDir() && !si.ModTime().After(di.ModTime()) && si.Size() == di.Size() {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(dst, data, 0o644)
}

func (s *Service) reconcile(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil
	}

	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	d, err := platform.ParseDescriptor(raw, platformKey)
	if err != nil {
		return err
	}

	cacheDir, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return err
	}
	shortDir := filepath.Join(cacheDir, "Shortcuts")
	if err := os.MkdirAll(shortDir, 0o755); err != nil {
		return err
	}
	www, err := platform.WwwrootDir()
	if err != nil {
		return err
	}

	ign := ignoreSet(d.Extras.ShortcutIgnore)

	var sourceFolders []string
	if strings.EqualFold(platformKey, "Steam") {
		sourceFolders = steamShortcutSourceDirs()
	} else {
		for _, folder := range d.Extras.ShortcutFolders {
			folder = strings.TrimSpace(folder)
			if folder == "" {
				continue
			}
			sourceFolders = append(sourceFolders, platform.ExpandWindowsPath(folder))
		}
	}

	for _, folder := range sourceFolders {
		if fi, err := os.Stat(folder); err != nil || !fi.IsDir() {
			continue
		}
		_ = filepath.WalkDir(folder, func(path string, de os.DirEntry, err error) error {
			if err != nil || de.IsDir() {
				return nil
			}
			name := de.Name()
			if !isShortcutFile(name) {
				return nil
			}
			if strings.Contains(strings.ToLower(name), "_ignored") {
				return nil
			}
			baseStem := removeShortcutExt(name)
			if strings.EqualFold(platformKey, "Steam") && strings.EqualFold(baseStem, "Steam") {
				return nil
			}
			if _, skip := ign[strings.ToLower(baseStem)]; skip {
				return nil
			}
			dst := filepath.Join(shortDir, name)
			_ = copyIfNewer(path, dst)
			return nil
		})
	}

	// Remove settings entries for missing or ignored files; collect files on disk
	lowToReal := map[string]string{}
	dirEntries, _ := os.ReadDir(shortDir)
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		low := strings.ToLower(n)
		if strings.Contains(low, "_ignored") {
			continue
		}
		if !isShortcutFile(n) {
			continue
		}
		lowToReal[low] = n
	}

	cur, err := loadEntries(platformKey)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{})
	var next []platform.GameShortcutEntry
	for _, e := range cur {
		fn := strings.TrimSpace(e.FileName)
		if fn == "" {
			continue
		}
		low := strings.ToLower(fn)
		if _, ok := lowToReal[low]; !ok {
			continue
		}
		seen[low] = struct{}{}
		next = append(next, e)
	}
	for low, real := range lowToReal {
		if _, ok := seen[low]; ok {
			continue
		}
		next = append(next, platform.GameShortcutEntry{FileName: real, Pinned: false})
	}

	if err := saveEntries(platformKey, next); err != nil {
		return err
	}

	// Icons (re-extract if missing or placeholder-sized PNG from earlier extraction bugs)
	const minShortcutIconBytes = 120
	for _, e := range next {
		fn := e.FileName
		outPath, err := iconDiskPath(platformKey, fn)
		if err != nil {
			continue
		}
		if fi, err := os.Stat(outPath); err == nil && !fi.IsDir() && fi.Size() >= minShortcutIconBytes {
			continue
		}
		if fi, err := os.Stat(outPath); err == nil && !fi.IsDir() && fi.Size() < minShortcutIconBytes {
			_ = os.Remove(outPath)
		}
		full := filepath.Join(shortDir, fn)
		_ = winutil.ExtractShortcutIcon(full, outPath)
	}

	// Main exe icon (Steam always; basic when ShortcutIncludeMainExe)
	includeMain := strings.EqualFold(platformKey, "Steam") ||
		(d.Extras.ShortcutIncludeMainExe != nil && *d.Extras.ShortcutIncludeMainExe)
	if includeMain && s.ps != nil {
		exe, err := s.ps.ResolvePlatformExeFullPath(platformKey)
		if err == nil && exe != "" {
			_, _ = exeicon.EnsureCached(platformKey, exe, www)
		}
	}

	s.emitUpdated(platformKey)
	return nil
}
