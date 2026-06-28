package platform

import (
	"encoding/json"
	"testing"
)

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

func TestCrashReportAutoSubmitDefaultsOnWhenMissing(t *testing.T) {
	settings := AppSettings{}

	normalizeAppSettingsDefaults(&settings, map[string]json.RawMessage{})

	if !settings.CrashReportAutoSubmit {
		t.Fatal("CrashReportAutoSubmit should default to true when the key is absent")
	}
}

func TestCrashReportAutoSubmitPreservesExplicitFalse(t *testing.T) {
	settings := AppSettings{CrashReportAutoSubmit: false}

	normalizeAppSettingsDefaults(&settings, map[string]json.RawMessage{
		"crashReportAutoSubmit": json.RawMessage("false"),
	})

	if settings.CrashReportAutoSubmit {
		t.Fatal("CrashReportAutoSubmit should preserve explicit false")
	}
}

func TestCrashReportAutoSubmitFalseMarshals(t *testing.T) {
	data, err := json.Marshal(AppSettings{CrashReportAutoSubmit: false})
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["crashReportAutoSubmit"]; !ok {
		t.Fatal("CrashReportAutoSubmit false should be written so it can round-trip")
	}
}

func TestNormalizeCommandPaletteHotkey(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "default for empty", value: "", want: "Ctrl+K"},
		{name: "legacy control alias", value: "Control + p", want: "Ctrl+P"},
		{name: "multiple modifiers", value: "ctrl+shift+k", want: "Ctrl+Shift+K"},
		{name: "function key", value: "Alt+F4", want: "Alt+F4"},
		{name: "single key rejected", value: "K", want: "Ctrl+K"},
		{name: "escape rejected", value: "Ctrl+Escape", want: "Ctrl+K"},
		{name: "duplicate modifier rejected", value: "Ctrl+Control+K", want: "Ctrl+K"},
		{name: "extra key rejected", value: "Ctrl+K+P", want: "Ctrl+K"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCommandPaletteHotkey(tt.value); got != tt.want {
				t.Fatalf("normalizeCommandPaletteHotkey(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
