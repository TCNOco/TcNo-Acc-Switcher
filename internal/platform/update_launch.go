package platform

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/updatecheck"
)

var launchCheckOnce sync.Once

const (
	AppUpdateAvailableEvent = "app-update-available"
	UpdateCheckFailedEvent  = "update-check-failed"

	wailsUpdateCheckTimeout = 60 * time.Second
	apiUpdateCheckTimeout   = 15 * time.Second
)

type UpdateAvailablePayload struct {
	Message     string `json:"message"`
	DownloadURL string `json:"downloadUrl"`
}

func emitAppUpdateAvailable(message string) {
	app := application.Get()
	if app == nil {
		return
	}
	_ = app.Event.Emit(AppUpdateAvailableEvent, UpdateAvailablePayload{
		Message:     message,
		DownloadURL: "https://github.com/TCNOco/TcNo-Acc-Switcher/releases/latest",
	})
}

func emitUpdateCheckFailed() {
	app := application.Get()
	if app == nil {
		return
	}
	_ = app.Event.Emit(UpdateCheckFailedEvent, true)
}

func (*PlatformService) NotifyLaunchUpdateCheck() {
	launchCheckOnce.Do(func() {
		go runLaunchUpdateCheck()
	})
}

func runLaunchUpdateCheck() {
	defer crashlog.Capture()
	time.Sleep(updatecheck.LaunchCheckDelay)

	exeDir, err := ResolveExeDir()
	if err != nil {
		return
	}
	s, err := loadSettings(exeDir)
	if err != nil || s.OfflineMode {
		return
	}

	go runLaunchPlatformsJSONCheck(exeDir)

	wailsCtx, wailsCancel := context.WithTimeout(context.Background(), wailsUpdateCheckTimeout)
	defer wailsCancel()
	if _, ok := tryWailsUpdateCheck(wailsCtx); ok {
		return
	}

	updatecheck.RunLaunchAPICheck(context.Background(), exeDir, appVersionFromBuildConfig(), emitAppUpdateAvailable, emitUpdateCheckFailed)
}

func (*PlatformService) CheckForUpdatesAndInstall() {
	go func() {
		defer crashlog.Capture()
		app := application.Get()
		if app == nil {
			return
		}
		exeDir, err := ResolveExeDir()
		if err != nil {
			return
		}
		s, err := loadSettings(exeDir)
		if err != nil || s.OfflineMode {
			return
		}
		if err := app.Updater.CheckAndInstall(context.Background()); err != nil {
			app.Logger.Error("update: CheckAndInstall", "error", err)
		}
	}()
}

// EnableAutoRestartAfterUpdate applies a staged update as soon as the Wails updater
// reaches the ready state. CheckAndInstall only downloads and stages; Restart swaps
// the binary and relaunches (see https://v3.wails.io/guides/updater/).
func EnableAutoRestartAfterUpdate(app *application.App) {
	if app == nil {
		return
	}
	app.Event.On(updater.EventUpdateReady, func(*application.CustomEvent) {
		go func() {
			defer crashlog.Capture()
			if err := app.Updater.Restart(context.Background()); err != nil {
				app.Logger.Error("update: Restart", "error", err)
			}
		}()
	})
}

func wailsReleaseMessage(rel *updater.Release) string {
	if rel == nil {
		return ""
	}
	if msg := strings.TrimSpace(rel.Notes); msg != "" {
		return msg
	}
	if msg := strings.TrimSpace(rel.Name); msg != "" {
		return msg
	}
	if v := strings.TrimSpace(rel.Version); v != "" {
		return "v" + strings.TrimPrefix(v, "v")
	}
	return ""
}

// tryWailsUpdateCheck uses the signed GitHub provider configured in gui.go.
// Returns ("available"|"up-to-date", true) on success, ("", false) when the
// updater is unavailable or the check failed (caller should fall back to API).
func tryWailsUpdateCheck(ctx context.Context) (string, bool) {
	app := application.Get()
	if app == nil {
		return "", false
	}
	if appclient.IsOfflineMode() {
		return "", false
	}

	rel, err := app.Updater.Check(ctx)
	if err != nil {
		if !errors.Is(err, updater.ErrNotConfigured) {
			app.Logger.Warn("update: wails check failed, falling back to API", "error", err)
		}
		return "", false
	}
	if rel == nil {
		return "up-to-date", true
	}
	emitAppUpdateAvailable(wailsReleaseMessage(rel))
	return "available", true
}

func runAPIUpdateCheck(ctx context.Context) string {
	return updatecheck.RunManualCheck(ctx, appVersionFromBuildConfig(), emitAppUpdateAvailable)
}

// CheckForUpdatesManually checks for updates when triggered from the UI.
// Uses the Wails signed GitHub updater first; falls back to the tcno.co API.
// Returns "available", "up-to-date", "offline", or "failed".
func (*PlatformService) CheckForUpdatesManually() string {
	exeDir, err := ResolveExeDir()
	if err != nil {
		return "failed"
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return "failed"
	}
	if s.OfflineMode {
		return "offline"
	}

	wailsCtx, wailsCancel := context.WithTimeout(context.Background(), wailsUpdateCheckTimeout)
	defer wailsCancel()
	if result, ok := tryWailsUpdateCheck(wailsCtx); ok {
		return result
	}

	apiCtx, apiCancel := context.WithTimeout(context.Background(), apiUpdateCheckTimeout)
	defer apiCancel()
	return runAPIUpdateCheck(apiCtx)
}
