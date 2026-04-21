package platform

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// i18nPlatformSep separates message key from {platform} value in i18n: payloads (U+001F).
const i18nPlatformSep = "\x1f"

// ActionBarStatusEvent is emitted with the footer status line text (empty clears).
const ActionBarStatusEvent = "action-bar-status"

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

// SetActionBarStatus is bound for the UI and other Go callers that go through services.
func (p *PlatformService) SetActionBarStatus(text string) {
	EmitActionBarStatus(text)
}
