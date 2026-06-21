//go:build !windows

package winutil

// CanKillProcesses is a no-op allow on non-Windows builds.
func CanKillProcesses(names []string, method ClosingMethod) (blocker string, ok bool) {
	return "", true
}
