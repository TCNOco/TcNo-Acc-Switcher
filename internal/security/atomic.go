package security

import (
	"os"
	"path/filepath"

	"TcNo-Acc-Switcher/internal/fsutil"
)

func writeFileAtomicDurable(path string, data []byte, perm os.FileMode) error {
	if err := fsutil.WriteFileAtomic(path, data, perm); err != nil {
		return err
	}
	syncDirBestEffort(filepath.Dir(path))
	return nil
}

func syncDirBestEffort(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}
