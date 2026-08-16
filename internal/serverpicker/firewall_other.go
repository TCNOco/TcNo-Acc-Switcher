//go:build !windows

package serverpicker

import "errors"

// errUnsupported is returned wherever the feature needs the Windows Firewall,
// which is the only backend the picker has.
var errUnsupported = errors.New("server picker is only supported on Windows")

func listBlockedGroupIDs() ([]string, error) { return nil, errUnsupported }

func applyBlock(string, []string, bool) error { return errUnsupported }
