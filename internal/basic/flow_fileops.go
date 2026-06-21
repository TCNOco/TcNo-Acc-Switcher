package basic

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/actionlog"
)

func isSharingViolationErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "being used by another process") ||
		strings.Contains(s, "the process cannot access the file") ||
		strings.Contains(s, "resource busy") ||
		strings.Contains(s, "sharing violation") ||
		strings.Contains(s, "lock violation")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		if isSharingViolationErr(err) {
			logFlow().Warn("copyFile: source locked (sharing violation)", "src", src, "dst", dst, "err", err)
		} else {
			logFlow().Debug("copyFile: open source failed", "src", src, "dst", dst, "err", err)
		}
		actionlog.Record("file:copy", src, dst, err)
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.Create(dst)
	if err != nil {
		if isSharingViolationErr(err) {
			logFlow().Warn("copyFile: destination locked (sharing violation)", "src", src, "dst", dst, "err", err)
		} else {
			logFlow().Debug("copyFile: create destination failed", "src", src, "dst", dst, "err", err)
		}
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		if isSharingViolationErr(err) {
			logFlow().Warn("copyFile: io.Copy hit sharing violation", "src", src, "dst", dst, "err", err)
		} else {
			logFlow().Debug("copyFile: io.Copy failed", "src", src, "dst", dst, "err", err)
		}
		actionlog.Record("file:copy", src, dst, err)
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	logFlow().Debug("copied file", "src", src, "dst", dst)
	actionlog.Record("file:copy", src, dst, nil)
	return nil
}

func copyFileToDir(src, dir string) error {
	return copyFile(src, filepath.Join(dir, filepath.Base(src)))
}

// copyOpErr wraps an error from copyFile/copyDir with a platform-key prefix
// and the live source path so user-facing toasts identify the failing operation.
func copyOpErr(platformKey, op, src string, err error) error {
	if err == nil {
		return nil
	}
	key := strings.TrimSpace(platformKey)
	if key == "" {
		key = "platform"
	}
	return fmt.Errorf("%s: %s %s: %w", key, op, src, err)
}

func copyDir(src, dst string) error {
	logFlow().Debug("copy directory tree", "src", src, "dst", dst)
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			if isSharingViolationErr(err) {
				logFlow().Warn("copyDir: walk hit sharing violation", "src", src, "dst", dst, "path", path, "err", err)
			} else {
				logFlow().Debug("copyDir: walk failed", "src", src, "dst", dst, "path", path, "err", err)
			}
			return fmt.Errorf("walk %s: %w", path, err)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}
		t := filepath.Join(dst, rel)
		if de.IsDir() {
			if err := os.MkdirAll(t, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", t, err)
			}
			return nil
		}
		return copyFile(path, t)
	})
}
