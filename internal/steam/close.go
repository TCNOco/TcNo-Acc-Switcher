package steam

import (
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/winutil"
)

// CloseSteam ends Steam the way switching accounts does, which means the client
// service and the helper processes as well: leaving those behind is what makes
// a half-closed Steam reappear with the previous account.
func (s *SteamService) CloseSteam() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	settings, err := LoadSettings()
	if err != nil {
		return err
	}
	method := winutil.ClosingMethod(settings.ClosingMethod)
	if err := winutil.ErrIfCannotKill(steamKillNames, method); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", PlatformKey)
		return err
	}
	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", PlatformKey)
	if err := winutil.KillByNameWithOpts(steamKillNames, method, nativeQuitOpts("")); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", PlatformKey)
		return err
	}
	platform.EmitActionBarStatus("")
	return nil
}
