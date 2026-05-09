package steam

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/tray"
	"TcNo-Acc-Switcher/internal/winutil"
)

var steamKillNames = []string{
	"steam.exe",
	"SERVICE:Steam Client Service",
	"steamwebhelper.exe",
	"GameOverlayUI.exe",
}

// SwapToAccount: empty steamID64 clears AutoLoginUser (Add New). personaState -1 uses Steam_OverrideState; < -1 skips localconfig persona edit. extraLaunchArgs append after settings argv.
func SwapToAccount(steamID64 string, personaState int, extraLaunchArgs []string) (err error) {
	defer func() {
		if err != nil {
			platform.EmitActionBarStatusI18n("Status_FailedLog")
			return
		}
		platform.EmitActionBarStatus("")
	}()
	platform.EmitActionBarStatusI18n("Status_Init")

	st, err := LoadSettings()
	if err != nil {
		return err
	}
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		return err
	}
	if root == "" {
		return fmt.Errorf("steam install folder not found")
	}

	if tr := strings.TrimSpace(steamID64); tr != "" && len(extraLaunchArgs) == 0 {
		if users, err := ParseLoginUsers(LoginUsersPath(root)); err == nil {
			if a := ActiveSessionSteamID64(users); a != "" && strings.EqualFold(strings.TrimSpace(a), tr) {
				return nil
			}
		}
	}

	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", "Steam")
	if err := winutil.ErrIfCannotKill(steamKillNames, winutil.ClosingMethod(st.ClosingMethod)); err != nil {
		platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatformFailed", "Steam")
		return err
	}
	if err := winutil.KillByName(steamKillNames, winutil.ClosingMethod(st.ClosingMethod), nil); err != nil {
		steamLog.Warn("kill steam processes", slog.Any("err", err))
	}

	platform.EmitActionBarStatusI18n("Status_ActionBar_UpdatingSteamLogin")
	pS := personaState
	if pS == -1 {
		pS = st.SteamOverrideState
	}

	if err := writeLoginUsersAndRegistry(root, steamID64); err != nil {
		return err
	}

	if err := setShowSteamSwitcher(root, st.ShowSteamSwitcher); err != nil {
		steamLog.Warn("config.vdf AlwaysShowUserChooser", slog.Any("err", err))
	}

	if pS >= 0 && strings.TrimSpace(steamID64) != "" {
		platform.EmitActionBarStatusI18n("Status_ActionBar_UpdatingSteamPersona")
		platform.EmitActionBarStatusI18nVars("Status_UpdatingFile", map[string]string{"file": "localconfig.vdf"})
		if err := setPersonaStateLocalConfig(root, steamID64, pS); err != nil {
			steamLog.Warn("localconfig ePersonaState", slog.Any("err", err))
		}
	}

	RecordTrayRecentAfterSwap(steamID64)
	if err := stats.IncrementSwitches("Steam"); err != nil {
		return err
	}
	platform.TriggerDiscordPresenceRefresh()

	if !st.AutoStart {
		tray.MaybeHideMainWindow()
		return nil
	}

	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", "Steam")
	args := buildSteamArgs(st, extraLaunchArgs)
	exe := filepath.Join(root, "steam.exe")
	opts := winutil.StartOpts{
		Admin:         st.RunAsAdmin,
		Method:        winutil.StartingMethod(strings.TrimSpace(st.StartingMethod)),
		HideWindow:    false,
		WorkingDir:    root,
		AsDesktopUser: winutil.IsProcessElevated() && !st.RunAsAdmin,
	}
	if err := winutil.Start(exe, args, opts); err != nil {
		return err
	}
	tray.MaybeHideMainWindow()
	return nil
}

func LaunchSteamOnly(extraLaunchArgs []string) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", "Steam")

	st, err := LoadSettings()
	if err != nil {
		return err
	}
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		return err
	}
	if root == "" {
		return fmt.Errorf("steam install folder not found")
	}
	args := buildSteamArgs(st, extraLaunchArgs)
	exe := filepath.Join(root, "steam.exe")
	opts := winutil.StartOpts{
		Admin:         st.RunAsAdmin,
		Method:        winutil.StartingMethod(strings.TrimSpace(st.StartingMethod)),
		HideWindow:    false,
		WorkingDir:    root,
		AsDesktopUser: winutil.IsProcessElevated() && !st.RunAsAdmin,
	}
	return winutil.Start(exe, args, opts)
}

func LaunchSteamOnlyAs(forceAdmin bool, extraLaunchArgs []string) error {
	defer platform.EmitActionBarStatus("")
	platform.EmitActionBarStatusI18nPlatform("Status_StartingPlatform", "Steam")

	st, err := LoadSettings()
	if err != nil {
		return err
	}
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		return err
	}
	if root == "" {
		return fmt.Errorf("steam install folder not found")
	}
	args := buildSteamArgs(st, extraLaunchArgs)
	exe := filepath.Join(root, "steam.exe")
	admin := st.RunAsAdmin
	if forceAdmin {
		admin = true
	}
	opts := winutil.StartOpts{
		Admin:         admin,
		Method:        winutil.StartingMethod(strings.TrimSpace(st.StartingMethod)),
		HideWindow:    false,
		WorkingDir:    root,
		AsDesktopUser: winutil.IsProcessElevated() && !admin,
	}
	return winutil.Start(exe, args, opts)
}

func buildSteamArgs(st Settings, extraLaunchArgs []string) []string {
	args := append([]string{}, platform.LaunchArgTokens(st.LaunchArguments)...)
	if len(extraLaunchArgs) > 0 {
		args = append(args, extraLaunchArgs...)
	}
	return args
}

func writeLoginUsersAndRegistry(steamRoot, selectedID64 string) error {
	loginPath := LoginUsersPath(steamRoot)
	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		return err
	}

	var autoUser string
	for i := range users {
		u := &users[i]
		u.MostRecent = "0"
		u.RememberPassword = "0"
		if strings.TrimSpace(selectedID64) == "" {
			continue
		}
		if u.SteamID64 == selectedID64 {
			u.MostRecent = "1"
			u.RememberPassword = "1"
			autoUser = u.AccountName
		}
	}

	if data, err := os.ReadFile(loginPath); err == nil && len(data) > 0 {
		_ = fsutil.WriteFileAtomic(strings.TrimSuffix(loginPath, ".vdf")+".vdf_last", data, 0o644)
	}

	kv := LoginUsersToKeyValue(users)
	out := KeyValueToText(kv)
	platform.EmitActionBarStatusI18nVars("Status_UpdatingFile", map[string]string{"file": "loginusers.vdf"})
	if err := fsutil.WriteFileAtomic(loginPath, out, 0o644); err != nil {
		return err
	}

	regBase := `HKCU\Software\Valve\Steam`
	platform.EmitActionBarStatusI18n("Status_UpdatingRegistry")
	if err := winutil.RegistryWrite(regBase+":AutoLoginUser", autoUser); err != nil {
		return err
	}
	if err := winutil.RegistryWrite(regBase+":RememberPassword", uint32(1)); err != nil {
		return err
	}
	return nil
}

func setShowSteamSwitcher(steamRoot string, show bool) error {
	path := filepath.Join(steamRoot, "config", "config.vdf")
	platform.EmitActionBarStatusI18nVars("Status_UpdatingFile", map[string]string{"file": "config.vdf"})
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(raw)
	val := "0"
	if show {
		val = "1"
	}
	lines := strings.Split(s, "\n")
	var out []string
	done := false
	for _, line := range lines {
		if strings.Contains(line, "AlwaysShowUserChooser") && strings.Contains(line, `"`) && !done {
			out = append(out, fmt.Sprintf(`				"AlwaysShowUserChooser"		"%s"`, val))
			done = true
			continue
		}
		out = append(out, line)
	}
	if !done {
		return nil
	}
	return fsutil.WriteFileAtomic(path, []byte(strings.Join(out, "\n")), 0o644)
}

func RemoveSteamAccountFromVDF(steamRoot, steamID64 string) error {
	loginPath := LoginUsersPath(steamRoot)
	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		return err
	}
	var kept []LoginUser
	for _, u := range users {
		if u.SteamID64 != steamID64 {
			kept = append(kept, u)
		}
	}
	if data, err := os.ReadFile(loginPath); err == nil && len(data) > 0 {
		_ = fsutil.WriteFileAtomic(strings.TrimSuffix(loginPath, ".vdf")+".vdf_last", data, 0o644)
	}
	kv := LoginUsersToKeyValue(kept)
	out := KeyValueToText(kv)
	return fsutil.WriteFileAtomic(loginPath, out, 0o644)
}
