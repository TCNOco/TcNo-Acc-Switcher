package stability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	buildinfo "TcNo-Acc-Switcher/build"
	"TcNo-Acc-Switcher/internal/api"
	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/logsanitize"
)

type ratePayload struct {
	UUID     string `json:"uuid"`
	Platform string `json:"platform"`
	Working  bool   `json:"working"`
}

type feedbackPayload struct {
	UUID     string `json:"uuid"`
	Kind     string `json:"kind"`
	Platform string `json:"platform,omitempty"`
	Text     string `json:"text"`
	Version  string `json:"version"`
	Log      string `json:"log,omitempty"`
}

func postJSON(url string, payload any) error {
	if appclient.IsOfflineMode() {
		return appclient.ErrOfflineMode
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", api.UserAgent(buildinfo.Version()))

	resp, err := appclient.Shared.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("stability: API rejected request", "url", url, "status", resp.StatusCode)
		return fmt.Errorf("api rejected request: %d", resp.StatusCode)
	}
	return nil
}

// SubmitRating sends a working/broken rating to the API asynchronously.
func SubmitRating(platform string, working bool) {
	go func() {
		defer crashlog.Capture()
		clientUUID, err := ClientUUID()
		if err != nil {
			slog.Warn("stability: client uuid", "err", err)
			return
		}
		if err := postJSON(api.StabilityRateURL(), ratePayload{
			UUID:     clientUUID,
			Platform: platform,
			Working:  working,
		}); err != nil {
			slog.Warn("stability: submit rating", "err", err)
		}
	}()
}

// SubmitFeedback sends issue or suggestion text to the API.
func SubmitFeedback(kind, platform, text string, attachLog bool) error {
	if appclient.IsOfflineMode() {
		return appclient.ErrOfflineMode
	}
	clientUUID, err := ClientUUID()
	if err != nil {
		return err
	}
	payload := feedbackPayload{
		UUID:     clientUUID,
		Kind:     kind,
		Platform: platform,
		Text:     text,
		Version:  buildinfo.Version(),
	}
	if attachLog && kind == "switch_issue" {
		payload.Log = logsanitize.ActionLogForUpload()
	}
	return postJSON(api.FeedbackURL(), payload)
}
