package platform

import "path/filepath"

// UserDataDirName is the folder next to the executable that holds per-user app data (for now)
// Will appear in %AppData% for installs, or stay here for portable installs.
// (same segment as [paths.DataRoot] without resolving exe dir).
const UserDataDirName = "TcNo Account Switcher"

// UserDataDir returns {exeDir}/TcNo Account Switcher/
func UserDataDir(exeDir string) string {
	return filepath.Join(exeDir, UserDataDirName)
}
