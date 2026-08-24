package serverpicker

import "log/slog"

func serverPickerLog() *slog.Logger {
	return slog.Default().With("component", "server-picker")
}
