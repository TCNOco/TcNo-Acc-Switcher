package steam

import (
	"TcNo-Acc-Switcher/internal/platform"
)

// CountSavedAccounts returns the number of Steam accounts in loginusers.vdf, or 0 when unavailable.
func CountSavedAccounts() int {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return 0
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return 0
	}
	st, err := LoadSettings()
	if err != nil {
		return 0
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return 0
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil || root == "" {
		return 0
	}
	users, err := ParseLoginUsers(LoginUsersPath(root))
	if err != nil {
		return 0
	}
	return len(users)
}
