package platform

import (
	"TcNo-Acc-Switcher/internal/streamer"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// StreamerModeChangedEvent carries a streamer.State whenever the effective answer,
// either preference, or the detected broadcaster changes.
const StreamerModeChangedEvent = "streamer-mode-changed"

func init() {
	streamer.SetChangeHook(func(s streamer.State) {
		app := application.Get()
		if app == nil {
			return
		}
		app.Event.Emit(StreamerModeChangedEvent, s)
	})
}

// InitStreamerMode seeds the detector from stored settings at startup.
func InitStreamerMode(s AppSettings) {
	streamer.Init(s.StreamerMode, s.AutoStreamerMode)
}

// GetStreamerState returns both preferences, what detection sees, the effective
// answer and the avatar salt in one call, so the UI hydrates without a round trip
// per field.
func (p *PlatformService) GetStreamerState() (streamer.State, error) {
	return streamer.Current(), nil
}
