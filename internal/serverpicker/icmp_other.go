//go:build !windows

package serverpicker

import "time"

// icmpEcho is Windows-only: the picker's firewall half does not exist elsewhere,
// so there is nothing to measure against.
func icmpEcho(string, time.Duration) (int, bool) { return 0, false }
