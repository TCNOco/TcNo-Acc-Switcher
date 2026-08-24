//go:build !windows && !linux

package steam

import "fmt"

// relaunchOutsideSteam has no macOS implementation yet, which only matters to
// someone who has added the switcher to Steam there.
func relaunchOutsideSteam(_ string, _ []string) error {
	return fmt.Errorf("leaving Steam's process tree is not implemented on this platform")
}
