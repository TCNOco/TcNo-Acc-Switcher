//go:build !windows

package platform

func resolveBackgroundSourcePath(string) (string, bool) {
	return "", false
}
