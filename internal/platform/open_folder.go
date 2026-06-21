package platform

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenPathInFileManager opens a directory in the OS file manager (Explorer, Finder, xdg-open).
func OpenPathInFileManager(path string) error {
	path = filepath.Clean(path)
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("explorer", path)
		return cmd.Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
