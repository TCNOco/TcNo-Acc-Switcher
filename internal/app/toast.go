package app

import "github.com/wailsapp/wails/v3/pkg/application"

type ToastPayload struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Duration int    `json:"duration"`
}

const toastEventName = "toast"

func EmitToast(typ, title, message string, durationMs int) {
	a := application.Get()
	if a == nil {
		return
	}
	a.Event.Emit(toastEventName, ToastPayload{
		Type:     typ,
		Title:    title,
		Message:  message,
		Duration: durationMs,
	})
}
