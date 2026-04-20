package darwin

// FindExeViaShortcuts is a stub on macOS (no Windows .lnk shortcuts).
func FindExeViaShortcuts(_ string, _ string) (string, bool) {
	return "", false
}
