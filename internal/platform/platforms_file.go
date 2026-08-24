package platform

import "runtime"

// PlatformsFileName is the platform catalog this OS reads and writes. The user
// data copy is named per-OS too, so a portable install on removable media does
// not have one OS overwrite the other's catalog.
func PlatformsFileName() string { return platformsFileNameFor(runtime.GOOS) }

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
