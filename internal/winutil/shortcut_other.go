//go:build !windows

package winutil

import "fmt"

func ReadLnkShortcut(lnkPath string) (target string, arguments string, iconLocation string, err error) {
	return "", "", "", fmt.Errorf("shortcuts: %w", ErrUnsupported)
}
