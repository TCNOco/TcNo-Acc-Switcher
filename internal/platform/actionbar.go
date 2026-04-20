package platform

import "github.com/wailsapp/wails/v3/pkg/application"

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

// SetActionBarStatus is bound for the UI and other Go callers that go through services.
func (p *PlatformService) SetActionBarStatus(text string) {
	EmitActionBarStatus(text)
}
