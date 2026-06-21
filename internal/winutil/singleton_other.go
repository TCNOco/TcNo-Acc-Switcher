//go:build !windows

package winutil

// TryAcquireSingleton — no single-instance enforcement on non-Windows builds.
func TryAcquireSingleton() (release func(), alreadyRunning bool, err error) {
	return func() {}, false, nil
}
