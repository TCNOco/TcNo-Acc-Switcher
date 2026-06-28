package platform

import (
	"strconv"
	"strings"
)

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
	if req.CommandPaletteHotkey != nil {
		s.CommandPaletteHotkey = normalizeCommandPaletteHotkey(*req.CommandPaletteHotkey)
		effects.dirty = true
	}

	return effects
}

func normalizeCommandPaletteHotkey(value string) string {
	const fallback = "Ctrl+K"
	compact := strings.ReplaceAll(strings.TrimSpace(value), " ", "")
	parts := strings.Split(compact, "+")
	if len(parts) < 2 {
		return fallback
	}
	modifiers := map[string]bool{}
	key := ""
	for _, part := range parts {
		if part == "" {
			return fallback
		}
		if modifier := normalizeCommandPaletteModifier(part); modifier != "" {
			if key != "" || modifiers[modifier] {
				return fallback
			}
			modifiers[modifier] = true
			continue
		}
		if key != "" {
			return fallback
		}
		key = normalizeCommandPaletteKey(part)
		if key == "" {
			return fallback
		}
	}
	if key == "" || (!modifiers["Ctrl"] && !modifiers["Alt"] && !modifiers["Shift"] && !modifiers["Meta"]) {
		return fallback
	}
	out := make([]string, 0, 5)
	for _, modifier := range []string{"Ctrl", "Alt", "Shift", "Meta"} {
		if modifiers[modifier] {
			out = append(out, modifier)
		}
	}
	out = append(out, key)
	return strings.Join(out, "+")
}

func normalizeCommandPaletteModifier(value string) string {
	switch strings.ToLower(value) {
	case "ctrl", "control":
		return "Ctrl"
	case "alt", "option":
		return "Alt"
	case "shift":
		return "Shift"
	case "meta", "cmd", "command", "win", "windows", "super":
		return "Meta"
	default:
		return ""
	}
}

func normalizeCommandPaletteKey(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	switch lower {
	case "esc", "escape":
		return ""
	case "space", "spacebar":
		return "Space"
	case "enter", "return":
		return "Enter"
	case "tab":
		return "Tab"
	case "backspace":
		return "Backspace"
	case "delete", "del":
		return "Delete"
	case "insert", "ins":
		return "Insert"
	case "home":
		return "Home"
	case "end":
		return "End"
	case "pageup", "pgup":
		return "PageUp"
	case "pagedown", "pgdown":
		return "PageDown"
	case "arrowup", "up":
		return "ArrowUp"
	case "arrowdown", "down":
		return "ArrowDown"
	case "arrowleft", "left":
		return "ArrowLeft"
	case "arrowright", "right":
		return "ArrowRight"
	}
	if len(lower) >= 2 && lower[0] == 'f' {
		n, err := strconv.Atoi(lower[1:])
		if err == nil && n >= 1 && n <= 24 {
			return "F" + strconv.Itoa(n)
		}
	}
	if len(value) == 1 {
		ch := value[0]
		if ch >= 'a' && ch <= 'z' {
			return strings.ToUpper(value)
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || strings.ContainsRune("`-=[]\\;',./", rune(ch)) {
			return strings.ToUpper(value)
		}
	}
	return ""
}

func stringsDefault(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
