//go:build !windows

package winutil

import "fmt"

func ExtractExeIcon(exePath, outPNG string) error {
	return fmt.Errorf("extract icon: %w", ErrUnsupported)
}
