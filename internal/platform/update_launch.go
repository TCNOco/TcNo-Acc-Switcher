package platform

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"TcNo-Acc-Switcher/internal/updatecheck"
)

const (
	// AppUpdateAvailableEvent signals the UI to show the update banner (payload ignored).
	AppUpdateAvailableEvent = "app-update-available"
	// UpdateCheckFailedEvent signals a failed reach to the update server (at most once per 24h from Go).
	UpdateCheckFailedEvent = "update-check-failed"
)

func emitAppUpdateAvailable() {
	app := application.Get()
	if app == nil {
		return
	}
	_ = app.Event.Emit(AppUpdateAvailableEvent, true)
}

func emitUpdateCheckFailed() {
	app := application.Get()
	if app == nil {
		return
	}
	_ = app.Event.Emit(UpdateCheckFailedEvent, true)
}

// Update checker. Skipped if Offline mode. Network errors may emit UpdateCheckFailedEvent at most once per 24 hours.
func (*PlatformService) NotifyLaunchUpdateCheck() {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return
	}
	updatecheck.StartLaunchCheck(exeDir, s.OfflineMode, appVersionFromBuildConfig(), emitAppUpdateAvailable, emitUpdateCheckFailed)
}
