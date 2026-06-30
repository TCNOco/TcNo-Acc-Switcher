package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const autostartIdentifier = "co.tcno.acc-switcher"

var errAutostartAppUnavailable = errors.New("application is not available")

func SetAutostartPreference(enabled bool) error {
	return SyncAutostartPreference(application.Get(), enabled)
}

func SyncAutostartPreference(app *application.App, enabled bool) error {
	err := applyWailsAutostart(app, enabled)
	if err == nil {
		return nil
	}
	if runtime.GOOS == "windows" && (errors.Is(err, errAutostartAppUnavailable) || errors.Is(err, application.ErrAutostartNotSupported)) {
		return syncWindowsAutostartFallback(enabled)
	}
	if errors.Is(err, application.ErrAutostartNotSupported) {
		return nil
	}
	return err
}

func applyWailsAutostart(app *application.App, enabled bool) error {
	if app == nil {
		return errAutostartAppUnavailable
	}
	if !enabled {
		return app.Autostart.Disable()
	}
	return app.Autostart.EnableWithOptions(application.AutostartOptions{
		Identifier: autostartIdentifier,
		Arguments:  []string{"-tray"},
	})
}

func syncWindowsAutostartFallback(enabled bool) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return winutil.SetRunAtStartupTray(filepath.Clean(self), enabled)
}
