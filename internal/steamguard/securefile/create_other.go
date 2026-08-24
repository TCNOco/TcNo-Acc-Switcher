//go:build !windows

package securefile

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var ErrHardeningUnsupported = errors.New("owner-only file protection is unavailable")

// CreateNew creates a file only its owner can read, failing if anything is there.
//
// O_EXCL is what makes this safe rather than a chmod afterwards: the file never
// exists at a wider mode, so there is no window in which another user could open
// it, and a path already occupied - including by a symlink someone planted - is
// refused outright.
func CreateNew(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, errors.Join(ErrHardeningUnsupported, err)
	}
	if err := verifyOwnerOnly(path, info.Mode().Perm()); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return file, nil
}

// CreateDirectoryNew creates a directory only its owner may enter. 0700 rather
// than 0600: without the execute bit the owner cannot open what is inside.
func CreateDirectoryNew(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		_ = os.Remove(path)
		return errors.Join(ErrHardeningUnsupported, err)
	}
	if err := verifyOwnerOnly(path, info.Mode().Perm()); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// verifyOwnerOnly checks what the filesystem actually recorded, not what was
// asked for. A umask can only clear bits, but a filesystem that ignores modes -
// a FAT stick, some network mounts - hands back a world-readable file, and that
// is worth failing on rather than storing a secret in it.
func verifyOwnerOnly(path string, perm os.FileMode) error {
	if perm&0o077 != 0 {
		return fmt.Errorf("%w: %s is mode %#o, which is not owner-only", ErrHardeningUnsupported, path, perm)
	}
	return nil
}
