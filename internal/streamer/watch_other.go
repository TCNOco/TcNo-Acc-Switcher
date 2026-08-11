//go:build !windows

package streamer

// setWatching is a no-op off Windows: the detection hooks are Win32-only, so auto
// streamer mode stays off and only the manual toggle applies.
func setWatching(_ bool) {}
