package steam

import (
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/tray"
)

// RecordTrayRecentAfterSwap updates Tray_Users.json for Steam after a successful login switch (+s: arg).
// steamID64 empty (Add New) is ignored.
func RecordTrayRecentAfterSwap(steamID64 string) {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return
	}
	st, err := LoadSettings()
	if err != nil || st.TrayAccNumber <= 0 {
		return
	}
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil || root == "" {
		return
	}
	users, err := ParseLoginUsers(LoginUsersPath(root))
	if err != nil {
		return
	}
	label := steamID64
	for _, u := range users {
		if u.SteamID64 != steamID64 {
			continue
		}
		label = trayLabelForUser(st, u)
		break
	}
	arg := "+s:" + steamID64
	_ = tray.AddUser(PlatformKey, arg, label, st.TrayAccNumber)
	tray.RefreshMenuIfSet()
}

func SyncTrayKnownAccounts() {
	st, err := LoadSettings()
	if err != nil || st.TrayAccNumber <= 0 {
		return
	}
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil || root == "" {
		return
	}
	users, err := ParseLoginUsers(LoginUsersPath(root))
	if err != nil {
		return
	}
	argNames := make(map[string]string, len(users))
	for _, u := range users {
		id := strings.TrimSpace(u.SteamID64)
		if id == "" {
			continue
		}
		argNames["+s:"+id] = trayLabelForUser(st, u)
	}
	_ = tray.SyncPlatformUsers(PlatformKey, argNames, st.TrayAccNumber)
}

func trayLabelForUser(st Settings, u LoginUser) string {
	if st.SteamTrayAccountName && strings.TrimSpace(u.AccountName) != "" {
		return strings.TrimSpace(u.AccountName)
	}
	if dn := strings.TrimSpace(CachedCommunityDisplayName(u.SteamID64)); dn != "" {
		return dn
	}
	return displayPersona(u)
}
