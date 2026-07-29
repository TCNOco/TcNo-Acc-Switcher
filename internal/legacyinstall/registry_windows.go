//go:build windows

package legacyinstall

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// legacyUninstallKeys are the Add/Remove Programs entries the C# NSIS installer
// wrote. It used a different key name from the Go installer ("TcNo-Acc-Switcher"
// against "TcNo Account Switcher"), so upgrading in place lists the app twice -
// the stale row carrying the old version string and pointing at whatever
// uninstaller now sits in the install folder.
//
// NSIS runs 32-bit, so its HKLM writes are redirected under WOW6432Node. The
// unredirected path is checked too, in case a build ever set SetRegView 64.
var legacyUninstallKeys = []string{
	`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\TcNo-Acc-Switcher`,
	`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\TcNo-Acc-Switcher`,
}

// PruneUninstallEntries deletes the C# uninstall entries that point at exeDir.
// Entries pointing elsewhere belong to another install and are left alone.
// Needs an elevated process; without one the delete fails with access denied.
func PruneUninstallEntries(exeDir string) ([]string, error) {
	exeDir = filepath.Clean(strings.TrimSpace(exeDir))
	if exeDir == "" {
		return nil, nil
	}

	var removed []string
	var errs []error
	for _, path := range legacyUninstallKeys {
		matches, err := uninstallEntryPointsAt(path, exeDir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !matches {
			continue
		}
		if err := registry.DeleteKey(registry.LOCAL_MACHINE, path); err != nil {
			if errors.Is(err, registry.ErrNotExist) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		removed = append(removed, `HKLM\`+path)
	}
	return removed, errors.Join(errs...)
}

func uninstallEntryPointsAt(path, exeDir string) (bool, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer key.Close()

	if loc, _, err := key.GetStringValue("InstallLocation"); err == nil {
		if loc = strings.TrimSpace(loc); loc != "" {
			return strings.EqualFold(filepath.Clean(loc), exeDir), nil
		}
	}
	// Older builds wrote no InstallLocation; fall back to the uninstaller path.
	uninst, _, err := key.GetStringValue("UninstallString")
	if err != nil {
		return false, nil
	}
	uninst = strings.Trim(strings.TrimSpace(uninst), `"`)
	if uninst == "" {
		return false, nil
	}
	return strings.EqualFold(filepath.Clean(filepath.Dir(uninst)), exeDir), nil
}
