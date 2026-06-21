package platform

import (
	"strings"

	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// i18nPlatformSep separates message key from {platform} value in i18n: payloads (U+001F).
const i18nPlatformSep = "\x1f"

// ActionBarStatusEvent is emitted with the footer status line text (empty clears).
const ActionBarStatusEvent = "action-bar-status"

func init() {
	winutil.SetStatusReporter(func(key string, vars map[string]string) {
		EmitActionBarStatusI18nVars(key, vars)
	})
}

// EmitActionBarStatus updates the app footer status from any package (e.g. account switching).
func EmitActionBarStatus(text string) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(ActionBarStatusEvent, text)
}

// EmitActionBarStatusI18n emits a message key for the frontend to translate (payload prefix i18n:).
func EmitActionBarStatusI18n(key string) {
	EmitActionBarStatus("i18n:" + key)
}

// EmitActionBarStatusI18nPlatform emits a key that uses {platform}, e.g. Status_ClosingPlatform / Status_StartingPlatform.
func EmitActionBarStatusI18nPlatform(key, platformName string) {
	platformName = strings.TrimSpace(platformName)
	if platformName == "" {
		EmitActionBarStatusI18n(key)
		return
	}
	EmitActionBarStatus("i18n:" + key + i18nPlatformSep + platformName)
}

// EmitActionBarStatusI18nVars emits an i18n status key with named template variables.
func EmitActionBarStatusI18nVars(key string, vars map[string]string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	var b strings.Builder
	b.WriteString("i18n:")
	b.WriteString(key)
	for name, value := range vars {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		b.WriteString(i18nPlatformSep)
		b.WriteString(name)
		b.WriteString(i18nPlatformSep)
		b.WriteString(value)
	}
	EmitActionBarStatus(b.String())
}

// SetActionBarStatus is bound for the UI and other Go callers that go through services.
func (p *PlatformService) SetActionBarStatus(text string) {
	EmitActionBarStatus(text)
}
