//go:build !windows

package winutil

func StartupTrayTaskExists() (bool, error) { return false, nil }

func SetStartupTrayTask(exePath string, enabled bool) error { return nil }
