package platform

import (
	"strings"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/stability"
)

// SubmitStabilityRating records whether an account switch worked for the given platform.
func (p *PlatformService) SubmitStabilityRating(platform string, working bool) error {
	if appclient.IsOfflineMode() {
		return appclient.ErrOfflineMode
	}
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return nil
	}
	stability.SubmitRating(platform, working)
	return nil
}

// SubmitFeedback sends a switch issue or feature suggestion to the API.
// kind must be "switch_issue" or "feature_suggestion".
func (p *PlatformService) SubmitFeedback(kind, platform, text string, attachLog bool) error {
	if appclient.IsOfflineMode() {
		return appclient.ErrOfflineMode
	}
	kind = strings.TrimSpace(kind)
	text = strings.TrimSpace(text)
	if kind == "" || text == "" {
		return nil
	}
	return stability.SubmitFeedback(kind, strings.TrimSpace(platform), text, attachLog)
}
