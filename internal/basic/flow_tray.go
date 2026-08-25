package basic

import (
	"strings"

	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/tray"
)

func recordBasicTrayRecent(platformKey, uniqueID string) {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	if platformKey == "" || uniqueID == "" {
		return
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil || ps.TrayAccNumber <= 0 {
		return
	}
	ids, err := readIDs(platformKey)
	if err != nil {
		return
	}
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	syncBasicTrayKnownAccounts(idx, platformKey, ids)
	name := strings.TrimSpace(ids[uniqueID])
	if name == "" {
		name = uniqueID
	}
	short := cli.ShortTokenForPlatform(idx, platformKey)
	if short == "" {
		return
	}
	arg := "+" + short + ":" + uniqueID
	_ = tray.AddUser(platformKey, arg, name, ps.TrayAccNumber)
	tray.RefreshMenuIfSet()
}

// syncBasicTrayKnownAccountsFor is the single-platform entry point, for callers
// acting on one account rather than sweeping every platform.
func syncBasicTrayKnownAccountsFor(platformKey string, ids map[string]string) {
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	syncBasicTrayKnownAccounts(idx, platformKey, ids)
}

// syncBasicTrayKnownAccounts takes the platform index rather than loading one:
// building it parses the whole catalog, and the caller sweeping every platform
// would otherwise pay that once per platform.
func syncBasicTrayKnownAccounts(idx *cli.PlatformIndex, platformKey string, ids map[string]string) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil || ps.TrayAccNumber <= 0 {
		return
	}
	short := cli.ShortTokenForPlatform(idx, platformKey)
	if short == "" {
		return
	}

	argNames := make(map[string]string, len(ids))
	for uniqueID, name := range ids {
		uniqueID = strings.TrimSpace(uniqueID)
		if uniqueID == "" {
			continue
		}
		argNames["+"+short+":"+uniqueID] = strings.TrimSpace(name)
	}
	_ = tray.SyncPlatformUsers(platformKey, argNames, ps.TrayAccNumber)
}

// SyncAllTrayKnownAccounts prunes the tray's recent-account lists against what
// each platform still has saved. It runs before the window is created.
func SyncAllTrayKnownAccounts() {
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	// One read of the tray file for the whole sweep. SyncPlatformUsers only ever
	// prunes a list that already exists - it returns without doing anything when
	// a platform has no tray entries - so a platform missing from here needs
	// none of the per-platform work below, and most installs have entries for
	// only one or two of the two dozen platforms.
	trayUsers, err := tray.LoadUsers()
	if err != nil {
		return
	}
	for _, platformKey := range idx.OrderedNames {
		if strings.EqualFold(strings.TrimSpace(platformKey), "Steam") {
			continue
		}
		if len(trayUsers[platformKey]) == 0 {
			continue
		}
		ids, err := readIDs(platformKey)
		if err != nil {
			continue
		}
		syncBasicTrayKnownAccounts(idx, platformKey, ids)
	}
}
