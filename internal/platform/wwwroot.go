package platform

import "path/filepath"

// WwwrootDir returns {ExeDir}/TcNo Account Switcher/wwwroot (same layout as internal/paths).
func WwwrootDir() (string, error) {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(exeDir, dataDirName, "wwwroot"), nil
}
