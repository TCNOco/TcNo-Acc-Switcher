package basic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
)

// validateRequiredLoginFiles distinguishes an active login from leftover account
// settings. A logged-out launcher must not replace a saved, authenticated session.
func validateRequiredLoginFiles(patterns []string, resolve func(string) string) error {
	for _, raw := range patterns {
		pattern := strings.TrimSpace(resolve(raw))
		if strings.TrimSpace(raw) == "" || pattern == "" {
			return fmt.Errorf("required login file has an empty or unmapped path")
		}
		files, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("required login file pattern %q: %w", pattern, err)
		}
		found := false
		for _, path := range files {
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("check required login file %q: %w", path, err)
			}
			if info.Mode().IsRegular() && info.Size() > 0 {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("required login file %q is missing or empty; sign in to the launcher and save the account again", pattern)
		}
	}
	return nil
}

func validateSavedLoginSession(fc FlowContext, accountName, uniqueID string) error {
	if len(fc.Descriptor.RequiredLoginFiles) == 0 {
		return nil
	}
	normalRoot, err := accountCacheDir(fc.PlatformKey, accountName)
	if err != nil {
		return err
	}
	root, cleanup, err := security.AccountRestoreDir(fc.PlatformKey, uniqueID, accountName, normalRoot)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := validateSavedLoginFiles(fc, root); err != nil {
		return fmt.Errorf("saved session for %q is incomplete: %w", accountName, err)
	}
	return nil
}

func validateSavedLoginFiles(fc FlowContext, root string) error {
	return validateRequiredLoginFiles(fc.Descriptor.RequiredLoginFiles, func(raw string) string {
		live := expandPlatformPath(raw, fc.Folder, fc.PathCtx)
		return mapLiveBuiltInToSavedPattern(fc.Descriptor, fc.Folder, fc.PathCtx, live, root)
	})
}

// Refresh known sessions before clearing them: a launcher can rotate its login
// token after Save Current restarts it. Never implicitly save an unknown account.
func resetLoginPreservingKnownSession(currentID string, accounts map[string]string, save func(string) error, reset func() error) error {
	if currentID = strings.TrimSpace(currentID); currentID != "" {
		if name := strings.TrimSpace(accounts[currentID]); name != "" {
			if err := save(name); err != nil {
				return err
			}
		}
	}
	return reset()
}

func validateLiveLoginFiles(d platform.Descriptor, ctx platform.PathTokenContext) error {
	return validateRequiredLoginFiles(d.RequiredLoginFiles, func(raw string) string {
		return expandPlatformPath(raw, ctx.PlatformFolder, ctx)
	})
}
