package platform

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const autostartIdentifier = "co.tcno.acc-switcher"

var errAutostartAppUnavailable = errors.New("application is not available")

func SetAutostartPreference(enabled, elevated bool) error {
	return SyncAutostartPreference(application.Get(), enabled, elevated)
}

// SyncAutostartPreference reconciles both Windows startup mechanisms with the
// preferences: a plain Run entry, and - when the app should always run as admin -
// a logon scheduled task, which is the only way to start elevated without a UAC
// prompt at every sign-in. Exactly one of the two is left in place.
func SyncAutostartPreference(app *application.App, enabled, elevated bool) error {
	if runtime.GOOS == "windows" {
		return syncWindowsAutostart(app, enabled, elevated)
	}
	err := applyWailsAutostart(app, enabled)
	if err == nil || errors.Is(err, application.ErrAutostartNotSupported) {
		return nil
	}
	return err
}

func syncWindowsAutostart(app *application.App, enabled, elevated bool) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self = filepath.Clean(self)

	wantTask := enabled && elevated
	// Registering and removing the task both need elevation, so this fails on
	// every unelevated run. That is not an error to report: the Run entry below
	// covers it, and the next elevated start reconciles the pair. Logged only so
	// a permanent failure - policy, a locked-down Task Scheduler - is traceable.
	taskErr := winutil.SetStartupTrayTask(self, wantTask)
	if taskErr != nil {
		log.Printf("autostart: scheduled task (elevated startup): %v", taskErr)
	}

	// A Run entry cannot start elevated, so it stays the fallback: a UAC prompt
	// at sign-in beats not starting at all.
	wantRunEntry := enabled && (!wantTask || taskErr != nil)
	return setWindowsRunEntry(app, wantRunEntry)
}

func setWindowsRunEntry(app *application.App, enabled bool) error {
	err := applyWailsAutostart(app, enabled)
	if err == nil {
		return nil
	}
	if errors.Is(err, errAutostartAppUnavailable) || errors.Is(err, application.ErrAutostartNotSupported) {
		return syncWindowsAutostartFallback(enabled)
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
