package steam

import (
	"TcNo-Acc-Switcher/internal/platform"
)

// CountSavedAccounts returns the number of Steam accounts the switcher knows,
// or 0 when unavailable. It counts the same union the account list shows, not
// just the rows Steam currently has.
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
	return len(knownAccountsForRoot(accountsRoot(root)))
}
