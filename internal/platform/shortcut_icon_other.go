//go:build !windows

package platform

func findStartMenuIconShortcut(entry PlatformEntry) (string, bool) {
	return "", false
}
