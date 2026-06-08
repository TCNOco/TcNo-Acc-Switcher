//go:build !windows

package winutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RestartSelf re-launches the current executable with extraArgs, then exits this process.
func RestartSelf(extraArgs []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)

	if singletonReleaser != nil {
		singletonReleaser()
	}

	args := append([]string{}, extraArgs...)
	cmd := exec.Command(self, args...)
	cmd.Dir = filepath.Dir(self)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	os.Exit(0)
	return nil
}
