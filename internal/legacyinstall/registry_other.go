//go:build !windows

package legacyinstall

// PruneUninstallEntries is Windows-only; the C# release never shipped elsewhere.
func PruneUninstallEntries(string) ([]string, error) { return nil, nil }
