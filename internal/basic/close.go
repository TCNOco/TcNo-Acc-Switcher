package basic

import (
	"errors"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/winutil"
)

// ClosePlatform ends the platform the same way switching accounts does: the
// executables its descriptor names, closed by the method its settings choose.
//
// Unlike a switch it never starts anything first. The Electron pre-kill step
// launches the platform so a window exists to close, which is worth it on the
// way to swapping an account and absurd when the user asked to exit.
func (b *BasicService) ClosePlatform(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return errors.New("platform is required")
	}
	descriptor, _, err := readDescriptor(platformKey)
	if err != nil {
		return err
	}
	if len(descriptor.ExesToEnd) == 0 {
		return nil
	}
	settings, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}
	method := winutil.ClosingMethod(settings.ClosingMethod)
	if err := winutil.ErrIfCannotKill(descriptor.ExesToEnd, method); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", platformKey)
		return err
	}
	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", platformKey)
	if err := winutil.KillByName(descriptor.ExesToEnd, method, nil); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", platformKey)
		return err
	}
	platform.EmitActionBarStatus("")
	return nil
}
