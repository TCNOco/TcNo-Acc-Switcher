package fsutil

import (
	"io"
	"os"
	"path/filepath"

	"TcNo-Acc-Switcher/internal/actionlog"
)

// CopyDir recursively copies a directory tree (files + subdirs, permissions not preserved for files).
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		t := filepath.Join(dst, rel)
		if de.IsDir() {
			return os.MkdirAll(t, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(t), 0o755); err != nil {
			return err
		}
		err = copyFileOne(path, t)
		if err != nil {
			return err
		}
		return nil
	})
}

func copyFileOne(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		actionlog.Record("file:copy", src, dst, err)
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "copytmp-")
	if err != nil {
		actionlog.Record("file:copy", src, dst, err)
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}
	_, err = io.Copy(tmp, in)
	if err != nil {
		cleanup()
		actionlog.Record("file:copy", src, dst, err)
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		actionlog.Record("file:copy", src, dst, err)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		actionlog.Record("file:copy", src, dst, err)
		return err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		actionlog.Record("file:copy", src, dst, err)
		return err
	}
	actionlog.Record("file:copy", src, dst, nil)
	return nil
}
