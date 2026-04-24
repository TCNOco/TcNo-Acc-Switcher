package shortcuts

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

var ErrNoShortcutFilesInDrop = errors.New("shortcuts: no .lnk or .url files in drop")

func (s *Service) ImportDroppedShortcuts(platformKey string, srcPaths []string) (int, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return 0, fmt.Errorf("missing platform key")
	}
	if len(srcPaths) == 0 {
		return 0, nil
	}

	var accepted []string
	for _, p := range srcPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		base := filepath.Base(p)
		if !isShortcutFile(base) {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		accepted = append(accepted, p)
	}
	if len(accepted) == 0 {
		return 0, ErrNoShortcutFilesInDrop
	}

	cacheDir, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return 0, err
	}
	shortDir := filepath.Join(cacheDir, "Shortcuts")
	if err := os.MkdirAll(shortDir, 0o755); err != nil {
		return 0, err
	}

	n := 0
	for _, src := range accepted {
		base := filepath.Base(src)
		ext := strings.ToLower(filepath.Ext(base))
		stemIn := strings.TrimSuffix(base, filepath.Ext(base))
		stem := paths.ShellShortcutBaseName(stemIn, 180)
		if stem == "" {
			stem = "shortcut"
		}
		destName := pickUniqueShortcutName(shortDir, stem, ext)
		destPath := filepath.Join(shortDir, destName)

		in, err := os.Open(src)
		if err != nil {
			return n, err
		}
		data, err := io.ReadAll(in)
		_ = in.Close()
		if err != nil {
			return n, err
		}
		if err := fsutil.WriteFileAtomic(destPath, data, 0o644); err != nil {
			return n, err
		}
		n++
	}

	if err := s.reconcile(platformKey); err != nil {
		return n, err
	}
	return n, nil
}

func pickUniqueShortcutName(shortDir, stem, ext string) string {
	try := stem + ext
	for i := 0; ; i++ {
		full := filepath.Join(shortDir, try)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			return try
		}
		if i == 0 {
			try = stem + " (1)" + ext
		} else {
			try = fmt.Sprintf("%s (%d)%s", stem, i+1, ext)
		}
	}
}
