//go:build !windows

package foreground

// WatchSteamGame is a no-op off Windows: the signal is a registry value Steam
// writes on Windows only.
func WatchSteamGame(onChange func(running bool)) func() {
	if onChange != nil {
		onChange(false)
	}
	return func() {}
}
