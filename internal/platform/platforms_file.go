package platform

import "runtime"

// PlatformsFileName is the platform catalog this OS reads and writes.
//
// The catalogs are per-OS files rather than one file with per-OS branches inside
// it: a descriptor is mostly install paths, process names and login files, and
// almost none of those survive a move between Windows and Linux. Splitting them
// also keeps a portable install on removable media from having one OS overwrite
// the other's catalog, which is the whole reason the user data copy is named
// after the OS too and not just the source file in the repository.
func PlatformsFileName() string { return platformsFileNameFor(runtime.GOOS) }

// platformsFileNameFor is the testable half of [PlatformsFileName].
//
// Anything that is not Windows or macOS gets the Linux catalog: the BSDs run the
// same native Steam layout under the same home-relative paths.
func platformsFileNameFor(goos string) string {
	switch goos {
	case "windows":
		return "Platforms.json"
	case "darwin":
		return "Platforms.mac.json"
	default:
		return "Platforms.linux.json"
	}
}
