package basic

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	logFlow().Debug("copied file", "src", src, "dst", dst)
	return nil
}

func copyFileToDir(src, dir string) error {
	return copyFile(src, filepath.Join(dir, filepath.Base(src)))
}

func copyDir(src, dst string) error {
	logFlow().Debug("copy directory tree", "src", src, "dst", dst)
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
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
