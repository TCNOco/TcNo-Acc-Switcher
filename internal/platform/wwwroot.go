package platform

import "path/filepath"

// WwwrootDir returns {UserDataDir}/wwwroot.
func WwwrootDir() (string, error) {
	ud, err := EffectiveUserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(ud, "wwwroot"), nil
}
