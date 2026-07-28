//go:build !windows

package vault

import "os"

type platformHardener struct{}

func defaultHardener() Hardener { return platformHardener{} }

func (platformHardener) HardenDir(path string) error  { return os.Chmod(path, 0o700) }
func (platformHardener) HardenFile(path string) error { return os.Chmod(path, 0o600) }
