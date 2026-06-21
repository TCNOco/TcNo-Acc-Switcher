//go:build !windows

package basic

import "errors"

func leveldbLockedDirROSnapshot(_ string) (string, error) {
	return "", errors.New("leveldb locked-directory snapshot is only supported on windows")
}
