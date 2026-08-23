package steam

import (
	"os"
	"path/filepath"
	"runtime"
)

// defaultFolderPath is where Steam installs itself on this OS, and the value a
// fresh SteamSettings.json carries.
//
// It matters more than a cosmetic default: ResolveInstallFolder trusts
// FolderPath ahead of everything else, so a Windows path shipped to a Linux user
// does not merely look wrong on the settings page - it is the answer the account
// list gets, and the reason the switcher would report looking for Steam under
// C:\Program Files (x86) on a machine with no C: drive.
func defaultFolderPath() string { return defaultFolderPathFor(runtime.GOOS) }

func defaultFolderPathFor(goos string) string {
	if goos == "windows" {
		return `C:\Program Files (x86)\Steam\`
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Better an empty default than a guess: resolution falls through to the
		// catalog, which can still find Steam.
		return ""
	}
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Steam")
	}
	return filepath.Join(home, ".local", "share", "Steam")
}
