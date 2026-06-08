//go:build !windows

package platform

func isSkippableCopyErrPlatform(err error) bool {
	return false
}
