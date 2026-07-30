package shortcuts

import (
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steam"
)

func loadEntries(platformKey string) ([]platform.GameShortcutEntry, error) {
	platformKey = strings.TrimSpace(platformKey)
	if strings.EqualFold(platformKey, "Steam") {
		st, err := steam.LoadSettings()
		if err != nil {
			return nil, err
		}
		return st.Shortcuts, nil
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return nil, err
	}
	return ps.Shortcuts, nil
}

func saveEntries(platformKey string, entries []platform.GameShortcutEntry) error {
	platformKey = strings.TrimSpace(platformKey)
	if strings.EqualFold(platformKey, "Steam") {
		return steam.UpdateSettings(func(st *steam.Settings) error {
			st.Shortcuts = entries
			return nil
		})
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}
	ps.Shortcuts = entries
	return platform.SavePlatformSettings(platformKey, ps)
}
