//go:build !windows

package winutil

const RunValueNameStartupTray = "TcNoAccSwitcher"

func RunAtStartupTrayCommand(exePath string) string { return "" }

func SetRunAtStartupTray(exePath string, enabled bool) error { return nil }

func SyncRunAtStartupTray(exePath string, want bool) error { return nil }
