//go:build !windows

package foreground

// Watch is a no-op off Windows: the detection is built on WinEvent hooks. The
// callback still fires once so callers do not have to special-case the state
// never arriving.
func Watch(onChange func(covered bool)) func() {
	if onChange != nil {
		onChange(false)
	}
	return func() {}
}
