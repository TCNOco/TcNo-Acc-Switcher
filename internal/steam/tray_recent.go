package steam

import (
	"os"
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
	pj, err := platform.ResolvePlatformsJSONPath(exeDir)
	if err != nil {
		return
	}
	raw, err := os.ReadFile(pj)
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
		if st.SteamTrayAccountName && strings.TrimSpace(u.AccountName) != "" {
			label = strings.TrimSpace(u.AccountName)
		} else {
			if dn := strings.TrimSpace(CachedCommunityDisplayName(steamID64)); dn != "" {
				label = dn
			} else {
				label = displayPersona(u)
			}
		}
		break
	}
	arg := "+s:" + steamID64
	_ = tray.AddUser(PlatformKey, arg, label, st.TrayAccNumber)
	tray.RefreshMenuIfSet()
}
