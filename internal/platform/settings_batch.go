package platform

import "strings"

type settingsBatchEffects struct {
	dirty                  bool
	statsEnabled           *bool
	offlineMode            *bool
	discordPresenceRefresh bool
}

func applySettingsBatchUpdate(s *AppSettings, req SettingsBatchUpdate) settingsBatchEffects {
	var effects settingsBatchEffects
	applyBool := func(ptr *bool, val *bool) {
		if val == nil {
			return
		}
		*ptr = *val
		effects.dirty = true
	}
	if req.OfflineMode != nil {
		s.OfflineMode = *req.OfflineMode
		if s.OfflineMode {
			s.DiscordRpc = false
			s.DiscordRpcShare = false
		}
		effects.offlineMode = req.OfflineMode
		effects.discordPresenceRefresh = true
		effects.dirty = true
	}
	applyBool(&s.ProtocolEnabled, req.ProtocolEnabled)
	applyBool(&s.ExitToTray, req.ExitToTray)
	if req.DiscordRpc != nil {
		next := *req.DiscordRpc
		if s.OfflineMode {
			next = false
		}
		s.DiscordRpc = next
		if !next {
			s.DiscordRpcShare = false
		}
		effects.discordPresenceRefresh = true
		effects.dirty = true
	}
	if req.DiscordRpcShare != nil {
		next := *req.DiscordRpcShare
		if s.OfflineMode || !s.DiscordRpc {
			next = false
		}
		s.DiscordRpcShare = next
		effects.discordPresenceRefresh = true
		effects.dirty = true
	}
	applyBool(&s.MinimizeOnSwitch, req.MinimizeOnSwitch)
	applyBool(&s.StartTrayWithWindows, req.StartTrayWithWindows)
	applyBool(&s.StartProgramCentered, req.StartProgramCentered)
	applyBool(&s.AnimationsEnabled, req.AnimationsEnabled)
	if req.StatsEnabled != nil {
		s.StatsEnabled = *req.StatsEnabled
		effects.statsEnabled = req.StatsEnabled
		effects.dirty = true
	}
	applyBool(&s.StatsShare, req.StatsShare)
	applyBool(&s.CrashReportAutoSubmit, req.CrashReportAutoSubmit)
	if req.Language != nil {
		s.Language = stringsDefault(*req.Language, "en-US")
		effects.dirty = true
	}
	if req.Theme != nil {
		nextTheme := sanitizeThemeID(*req.Theme)
		if s.Theme != nextTheme {
			s.ThemeAccentPreset = ""
			s.ThemeAccentCustom = ""
		}
		s.Theme = nextTheme
		effects.dirty = true
	}
	if req.ThemeAccentPreset != nil {
		s.ThemeAccentPreset = sanitizeThemeAccentPreset(*req.ThemeAccentPreset)
		effects.dirty = true
	}
	if req.ThemeAccentCustom != nil {
		s.ThemeAccentCustom = sanitizeHexColor(*req.ThemeAccentCustom)
		effects.dirty = true
	}

	return effects
}

func stringsDefault(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
