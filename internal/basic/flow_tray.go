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

// syncBasicTrayKnownAccountsFor is the single-platform entry point for callers
// that do not already hold a platform index.
func syncBasicTrayKnownAccountsFor(platformKey string, ids map[string]string) {
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	syncBasicTrayKnownAccounts(idx, platformKey, ids)
}

// syncBasicTrayKnownAccounts takes the platform index rather than loading one:
// building it parses the whole catalog, which a sweep would pay per platform.
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
// each platform still has saved.
func SyncAllTrayKnownAccounts() {
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	// One read of the tray file for the whole sweep: SyncPlatformUsers only prunes
	// lists that already exist, so a platform with no tray entries needs none of the
	// per-platform work below.
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
