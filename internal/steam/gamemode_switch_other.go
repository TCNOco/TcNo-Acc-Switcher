//go:build !linux

package steam

import "fmt"

// startSwitchHelper is unreachable off Linux: gamescope is a Linux compositor,
// so shouldHandOffSwitch is always false there.
func startSwitchHelper(_ string, _ []string) error {
	return fmt.Errorf("handing off a switch is only implemented on Linux")
}
