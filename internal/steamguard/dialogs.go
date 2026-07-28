package steamguard

import (
	"log/slog"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// dialogCancelled reports whether a native file dialog failure is really the
// user pressing Cancel. Wails v3 on Windows returns the go-common-file-dialog
// sentinel (cfd.ErrorCancelled, "cancelled by user") verbatim from
// PromptForSingleSelection/PromptForMultipleSelection, but that package lives
// under wails/v3/internal so the sentinel cannot be compared by identity.
// Matching the message is the only option available to callers.
func dialogCancelled(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "cancelled by user") ||
		strings.Contains(message, "canceled by user")
}

// dialogOwnerWindow returns the main window so native dialogs are modal to it
// and cannot open behind the frameless main window. A nil result is valid and
// simply means "no owner".
func dialogOwnerWindow() application.Window {
	app := application.Get()
	if app == nil || app.Window == nil {
		return nil
	}
	window, ok := app.Window.GetByName(mainWindowName)
	if !ok || window == nil {
		return nil
	}
	return window
}

func dialogLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard.dialog")
}

// logDialogOutcome records cancellations and failures without ever touching the
// selected paths beyond whether one was chosen.
func logDialogOutcome(dialog string, selected bool, err error) {
	log := dialogLogger()
	switch {
	case dialogCancelled(err):
		log.Debug("native dialog cancelled", "dialog", dialog)
	case err != nil:
		log.Warn("native dialog failed", "dialog", dialog, "error", err)
	case !selected:
		log.Debug("native dialog closed without a selection", "dialog", dialog)
	}
}
