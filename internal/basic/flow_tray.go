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
	syncBasicTrayKnownAccounts(platformKey, ids)
	name := strings.TrimSpace(ids[uniqueID])
	if name == "" {
		name = uniqueID
	}
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	short := cli.ShortTokenForPlatform(idx, platformKey)
	if short == "" {
		return
	}
	arg := "+" + short + ":" + uniqueID
	_ = tray.AddUser(platformKey, arg, name, ps.TrayAccNumber)
	tray.RefreshMenuIfSet()
}

func syncBasicTrayKnownAccounts(platformKey string, ids map[string]string) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil || ps.TrayAccNumber <= 0 {
		return
	}
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
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

func SyncAllTrayKnownAccounts() {
	idx, err := cli.LoadPlatformIndex()
	if err != nil {
		return
	}
	for _, platformKey := range idx.OrderedNames {
		if strings.EqualFold(strings.TrimSpace(platformKey), "Steam") {
			continue
		}
		ids, err := readIDs(platformKey)
		if err != nil {
			continue
		}
		syncBasicTrayKnownAccounts(platformKey, ids)
	}
}
