//go:build windows

package steam

import (
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

// writeAutoLogin points Steam at the account to sign in as. On Windows that is
// the registry; steamRoot is unused.
func writeAutoLogin(steamRoot, autoUser string) error {
	const regBase = `HKCU\Software\Valve\Steam`
	platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
	if err := winutil.RegistryWrite(regBase+":AutoLoginUser", autoUser); err != nil {
		return err
	}
	return winutil.RegistryWrite(regBase+":RememberPassword", uint32(1))
}
