package platform

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"TcNo-Acc-Switcher/internal/updatecheck"
)

const (
	AppUpdateAvailableEvent = "app-update-available"
	UpdateCheckFailedEvent  = "update-check-failed"
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

func (*PlatformService) CheckForUpdatesAndInstall() {
	go func() {
		app := application.Get()
		if app == nil {
			return
		}
		if err := app.Updater.CheckAndInstall(context.Background()); err != nil {
			app.Logger.Error("update: CheckAndInstall", "error", err)
		}
	}()
}
