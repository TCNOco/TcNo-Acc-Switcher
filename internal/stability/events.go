package stability

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

const stabilityPromptEvent = "stability-prompt"

type StabilityPromptPayload struct {
	Platform string `json:"platform"`
}

// EmitStabilityPrompt notifies the frontend to show the rating popup.
func EmitStabilityPrompt(platform string) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(stabilityPromptEvent, StabilityPromptPayload{
		Platform: platform,
	})
}
