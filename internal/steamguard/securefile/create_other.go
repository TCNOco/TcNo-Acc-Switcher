//go:build !windows

package securefile

import (
	"errors"
	"os"
)

var ErrHardeningUnsupported = errors.New("owner-only file protection is unavailable")

func CreateNew(string) (*os.File, error) { return nil, ErrHardeningUnsupported }

func CreateDirectoryNew(string) error { return ErrHardeningUnsupported }
