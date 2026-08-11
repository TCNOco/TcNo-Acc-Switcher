package steambrowser

import (
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// logger records session-window lifecycle. Nothing secret reaches it: window
// ids and Steam IDs only, never a token, a cookie value or a page's content.
//
// Placement and visibility failures are silent by nature - the window still
// opens, it just shows nothing - so this is the only way to tell a view that
// failed to appear from one that appeared and loaded an empty page.
func logger() *slog.Logger {
	if app := application.Get(); app != nil && app.Logger != nil {
		return app.Logger.With("component", "steambrowser")
	}
	return slog.Default().With("component", "steambrowser")
}
