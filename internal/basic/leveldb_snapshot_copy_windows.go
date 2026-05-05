//go:build windows

package basic

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// leveldbLockedDirROSnapshot copies a LevelDB directory into a new folder under the
// system temp directory so it can be opened read-only while the original is locked
// by another process (e.g. Chromium Local Storage). The LOCK file is omitted so the
// reader creates its own lock on the copy. Files are opened with wide sharing so
// in-use data files can often still be read.
func leveldbLockedDirROSnapshot(src string) (_ string, err error) {
	src = filepath.Clean(src)
	tmp, err := os.MkdirTemp("", "tcno-leveldb-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tmp)
		}
	}()

	err = filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, e0 := filepath.Rel(src, path)
		if e0 != nil {
			return e0
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		dstPath := filepath.Join(tmp, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o700)
		}
		if strings.EqualFold(d.Name(), "LOCK") {
			return nil
		}
		return copyFileWideShare(path, dstPath)
	})
	if err != nil {
		return "", fmt.Errorf("copy leveldb tree: %w", err)
	}
	if _, stat := os.Stat(filepath.Join(tmp, "CURRENT")); stat != nil {
		return "", fmt.Errorf("copy leveldb: missing CURRENT: %w", stat)
	}
	return tmp, nil
}

func copyFileWideShare(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	src16, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(
		src16,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(h), src)
	defer f.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		return err
	}
	return out.Sync()
}
