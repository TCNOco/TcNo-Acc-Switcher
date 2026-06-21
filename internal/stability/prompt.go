package stability

import (
	"log/slog"

	"TcNo-Acc-Switcher/internal/appclient"
)

// OnSuccessfulSwitch increments the per-platform switch counter and may emit a rating prompt.
func OnSuccessfulSwitch(platformKey string) {
	mu.Lock()
	count, err := incrementSwitchLocked(platformKey)
	mu.Unlock()
	if err != nil {
		slog.Warn("stability: increment switch counter", "err", err, "platform", platformKey)
		return
	}
	if appclient.IsOfflineMode() {
		return
	}
	if ShouldPromptForCount(count) {
		EmitStabilityPrompt(platformKey)
	}
}