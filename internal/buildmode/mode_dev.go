//go:build !production

package buildmode

// IsDebugBuild reports whether this binary was built in non-production mode.
func IsDebugBuild() bool {
	return true
}
