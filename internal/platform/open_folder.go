package platform

import (
	"os/exec"
	"path/filepath"
	"runtime"

	"TcNo-Acc-Switcher/internal/winutil"
)

// OpenPathInFileManager opens a directory in the OS file manager (Explorer, Finder, xdg-open).
func OpenPathInFileManager(path string) error {
	path = filepath.Clean(path)
	switch runtime.GOOS {
	case "windows":
		return winutil.Start("explorer.exe", []string{path}, winutil.StartOpts{})
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
