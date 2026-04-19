package main

import "github.com/wailsapp/wails/v3/pkg/application"

// ToastPayload is emitted to the frontend as the "toast" custom event (Wails v3).
type ToastPayload struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Duration int    `json:"duration"` // milliseconds; 0 means use default (5000) on the frontend
}

const toastEventName = "toast"

// EmitToast shows a toast from Go. Callable from any goroutine after application.New.
// durationMs <= 0 uses the frontend default (5000 ms).
func EmitToast(typ, title, message string, durationMs int) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(toastEventName, ToastPayload{
		Type:     typ,
		Title:    title,
		Message:  message,
		Duration: durationMs,
	})
}
