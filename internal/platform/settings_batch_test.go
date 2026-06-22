package platform

import "testing"

func TestApplySettingsBatchUpdateOfflineDisablesDiscord(t *testing.T) {
	on := true
	settings := AppSettings{DiscordRpc: true, DiscordRpcShare: true}

	effects := applySettingsBatchUpdate(&settings, SettingsBatchUpdate{OfflineMode: &on})

	if !settings.OfflineMode || settings.DiscordRpc || settings.DiscordRpcShare {
		t.Fatalf("offline update left settings inconsistent: %#v", settings)
	}
	if effects.offlineMode == nil || !*effects.offlineMode || !effects.discordPresenceRefresh || !effects.dirty {
		t.Fatalf("offline update effects = %#v", effects)
	}
}

func TestApplySettingsBatchUpdateGuardsDiscordShare(t *testing.T) {
	on := true
	settings := AppSettings{DiscordRpc: false}

	effects := applySettingsBatchUpdate(&settings, SettingsBatchUpdate{DiscordRpcShare: &on})

	if settings.DiscordRpcShare {
		t.Fatal("discord share should stay disabled when Discord RPC is disabled")
	}
	if !effects.discordPresenceRefresh || !effects.dirty {
		t.Fatalf("discord share effects = %#v", effects)
	}
}

func TestApplySettingsBatchUpdateSanitizesThemeAndAccent(t *testing.T) {
	theme := "New_Theme"
	badAccent := "#not-ok"
	settings := AppSettings{
		Theme:             "Old",
		ThemeAccentPreset: "windows",
		ThemeAccentCustom: "#112233",
	}

	applySettingsBatchUpdate(&settings, SettingsBatchUpdate{
		Theme:             &theme,
		ThemeAccentCustom: &badAccent,
	})

	if settings.Theme != "New_Theme" {
		t.Fatalf("Theme = %q, want New_Theme", settings.Theme)
	}
	if settings.ThemeAccentPreset != "" || settings.ThemeAccentCustom != "" {
		t.Fatalf("accent fields = (%q, %q), want both cleared", settings.ThemeAccentPreset, settings.ThemeAccentCustom)
	}
}

func TestApplySettingsBatchUpdateStatsEffect(t *testing.T) {
	off := false
	settings := AppSettings{StatsEnabled: true}

	effects := applySettingsBatchUpdate(&settings, SettingsBatchUpdate{StatsEnabled: &off})

	if settings.StatsEnabled {
		t.Fatal("StatsEnabled should be false")
	}
	if effects.statsEnabled == nil || *effects.statsEnabled {
		t.Fatalf("stats effect = %#v", effects.statsEnabled)
	}
}
