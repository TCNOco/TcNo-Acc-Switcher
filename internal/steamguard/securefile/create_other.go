//go:build !windows

package securefile

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var ErrHardeningUnsupported = errors.New("owner-only file protection is unavailable")

// CreateNew creates a file no one but its owner can read, failing if anything is
// already there.
//
// O_EXCL is what makes this safe rather than a chmod after the fact: the file
// never exists at a wider mode, so there is no window in which another user
// could open it. O_NOFOLLOW is redundant next to O_EXCL, which already refuses a
// path occupied by a symlink, and is kept because the cost is nothing and the
// thing being protected is a Steam Guard secret.
//
// The Windows side also asks for write-through. There is no equivalent here on
// purpose: durability across a crash is the vault journal's job, and opening
// every secret file O_SYNC would pay for it twice.
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

// CreateDirectoryNew creates a directory only its owner may enter, failing if
// anything is already there. 0700 rather than 0600: without the execute bit the
// owner cannot open what is inside it.
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

// verifyOwnerOnly rejects any group or other permission bit.
//
// It checks what the filesystem actually recorded, not what was requested. A
// umask can only clear bits, so a stricter mode than asked for is still correct,
// but a filesystem that ignores modes outright - a FAT stick, some network
// mounts - hands back a world-readable file, and that is worth failing on
// rather than storing a secret in it.
func verifyOwnerOnly(path string, perm os.FileMode) error {
	if perm&0o077 != 0 {
		return fmt.Errorf("%w: %s is mode %#o, which is not owner-only", ErrHardeningUnsupported, path, perm)
	}
	return nil
}
