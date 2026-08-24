package steam

import (
	"os"
	"path/filepath"
	"runtime"
)

// defaultFolderPath is where Steam installs itself on this OS.
//
// ResolveInstallFolder trusts FolderPath ahead of everything else, so a wrong
// default here is not cosmetic - it is the answer the account list gets.
func defaultFolderPath() string { return defaultFolderPathFor(runtime.GOOS) }

func defaultFolderPathFor(goos string) string {
	if goos == "windows" {
		return `C:\Program Files (x86)\Steam\`
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Empty rather than a guess: resolution falls through to the catalog.
		return ""
	}
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Steam")
	}
	return filepath.Join(home, ".local", "share", "Steam")
}
