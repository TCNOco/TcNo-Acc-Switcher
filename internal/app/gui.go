package app

import (
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	buildinfo "TcNo-Acc-Switcher/build"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/buildmode"
	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/discordrpc"
	"TcNo-Acc-Switcher/internal/ipc"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/shortcuts"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/tray"
	"TcNo-Acc-Switcher/internal/updatecheck"
	"TcNo-Acc-Switcher/internal/updatertheme"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

type RunGUIParams struct {
	Parsed           cli.Parsed
	GuiSettings      platform.AppSettings
	Services         []application.Service
	Dispatch         *Dispatch
	DiscordRPC       *discordrpc.Manager
	CrashSubmitted   bool
	StartupToast     string
	EmbeddedAssets   fs.FS
	TrayIconPNG      []byte
	UpdaterPublicKey []byte
	Done             chan struct{}
}

func ResolvedLogLevel(p cli.Parsed) slog.Level {
	lvl := p.EffectiveSlogLevel()
	if buildmode.IsDebugBuild() && !p.LogLevelSet {
		lvl = slog.LevelDebug
	}
	return lvl
}

func mainWindowOptions(guiSettings platform.AppSettings, parsed cli.Parsed) application.WebviewWindowOptions {
	winOpts := application.WebviewWindowOptions{
		Name:      "main",
		Title:     "TcNo Account Switcher",
		MinWidth:  760,
		MinHeight: 520,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour:           application.NewRGB(27, 38, 54),
		URL:                        "/",
		Frameless:                  true,
		EnableFileDrop:             true,
		DevToolsEnabled:            true,
		DefaultContextMenuDisabled: false,
		KeyBindings: map[string]func(application.Window){
			"Ctrl+Shift+I": func(window application.Window) { window.OpenDevTools() },
			"F11":          func(window application.Window) { window.ToggleFullscreen() },
		},
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionCamera:        application.PermissionDeny,
			application.PermissionMicrophone:    application.PermissionDeny,
			application.PermissionGeolocation:   application.PermissionDeny,
			application.PermissionNotifications: application.PermissionDeny,
			application.PermissionClipboardRead: application.PermissionDeny,
		},
	}
	if guiSettings.StartProgramCentered {
		winOpts.InitialPosition = application.WindowCentered
	} else {
		winOpts.InitialPosition = application.WindowXY
		winOpts.X = 96
		winOpts.Y = 96
	}
	if parsed.StartInTray {
		winOpts.Hidden = true
	}
	return winOpts
}

func githubUpdaterConfig(guiSettings platform.AppSettings) github.Config {
	return github.Config{
		Repository:    "TCNOco/TcNo-Acc-Switcher",
		Prerelease:    guiSettings.PrereleaseUpdates,
		ChecksumAsset: "SHA256SUMS",
		AssetMatcher:  updatecheck.GitHubAssetMatcher,
	}
}

func RunGUI(params RunGUIParams) {
	parsed := params.Parsed
	guiSettings := params.GuiSettings
	disp := params.Dispatch

	if parsed.Kind == cli.KindOpenPage {
		platform.SetStartupNavigateHint(parsed.RouteJSONForOpenPage())
	}

	syncProtocolRegistration()

	if err := stats.IncrementLaunchCount(); err != nil {
		log.Printf("stats launch count: %v", err)
	}
	go stats.MustTryUploadDaily(guiSettings.StatsEnabled, guiSettings.StatsShare, guiSettings.OfflineMode)
	params.DiscordRPC.Start()

	wailsLvl := ResolvedLogLevel(parsed)
	if !parsed.LogLevelSet && wailsLvl < slog.LevelInfo {
		wailsLvl = slog.LevelInfo
	}
	wailsLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: wailsLvl}))
	notifier := notifications.New()
	platform.SetNativeNotifier(notifier)
	services := append([]application.Service{}, params.Services...)
	services = append(services, application.NewService(notifier))

	var wailsApp *application.App
	appOpts := application.Options{
		Name:        "TcNo Account Switcher",
		Description: "A Superfast open-source account switcher",
		LogLevel:    wailsLvl,
		Logger:      wailsLogger,
		Services:    services,
		Assets: application.AssetOptions{
			Handler: newCompositeAssetHandler(params.EmbeddedAssets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "co.tcno.acc-switcher",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				handleForwardedCLI(wailsApp, disp, argvWithoutExecutable(data.Args))
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	}
	if runtime.GOOS == "windows" {
		if cacheDir, err := paths.WebViewCacheDir(); err != nil {
			log.Printf("webview cache dir: %v", err)
		} else if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			log.Printf("webview cache dir: %v", err)
		} else {
			configureWindowsWebViewCache(&appOpts, cacheDir)
		}
	}

	wailsApp = application.New(appOpts)
	if err := platform.SyncAutostartPreference(wailsApp, guiSettings.StartTrayWithWindows); err != nil {
		wailsApp.Logger.Warn("autostart sync", "error", err)
	}

	currentVersion := buildinfo.Version()

	if currentVersion != "" && !guiSettings.OfflineMode {
		gh, err := updatecheck.NewSignedGitHubProvider(githubUpdaterConfig(guiSettings), ".exe.sig")
		if err != nil {
			wailsApp.Logger.Error("updater: provider", "error", err)
		} else {
			updaterWindow := updatertheme.NewBuiltinWindow()
			updatertheme.SetWindow(updaterWindow)
			if err := wailsApp.Updater.Init(updater.Config{
				CurrentVersion: currentVersion,
				Providers:      []updater.Provider{gh},
				PublicKey:      params.UpdaterPublicKey,
				Window:         updaterWindow,
			}); err != nil {
				wailsApp.Logger.Error("updater: init", "error", err)
			} else {
				platform.EnableAutoRestartAfterUpdate(wailsApp)
			}
		}

	}

	if params.CrashSubmitted {
		EmitToast("success", "i18n:Toast_CrashReported", "", 0)
	}
	if toast := strings.TrimSpace(params.StartupToast); toast != "" {
		EmitToast("success", toast, "", 6000)
	}
	var ipcStop func()
	wailsApp.OnShutdown(func() {
		params.DiscordRPC.Stop()
		if ipcStop != nil {
			ipcStop()
		}
		if params.Done != nil {
			close(params.Done)
		}
	})

	ipcStop, err := ipc.StartGUIServer(func(argv []string) {
		handleForwardedCLI(wailsApp, disp, argv)
	})
	if err != nil {
		log.Printf("ipc server: %v", err)
	}

	winOpts := mainWindowOptions(guiSettings, parsed)
	win := wailsApp.Window.NewWithOptions(winOpts)
	registerNotificationResponseHandler(wailsApp, win, notifier)
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		details := event.Context().DropTargetDetails()
		wailsApp.Event.Emit(shortcuts.FilesDroppedEvent, shortcuts.FilesDroppedPayload{
			Files: files,
			Target: shortcuts.FileDropTargetDetails{
				ElementID: details.ElementID,
				ClassList: append([]string(nil), details.ClassList...),
				X:         details.X,
				Y:         details.Y,
			},
		})
	})

	trayMgr := tray.NewManager(wailsApp, win, tray.Deps{
		SwapBasic: func(platformKey, uniqueID string) error {
			return disp.BasicSvc.SwapToAccount(platformKey, uniqueID, nil)
		},
		SwapSteam: func(steamID64 string, personaState int) error {
			return disp.SteamSvc.SwapToSteamAccount(steamID64, personaState, nil)
		},
	})
	trayMgr.RegisterCloseHook()
	tray.SetMenuRefresh(trayMgr.RefreshMenu)
	if !security.AppLocked() {
		basic.SyncAllTrayKnownAccounts()
		steam.SyncTrayKnownAccounts()
	}
	trayMgr.Start(params.TrayIconPNG)

	basic.SetLiveAccountIDResolver(func(platformKey string) (string, error) {
		if strings.EqualFold(strings.TrimSpace(platformKey), "Steam") {
			return steam.CurrentLiveSteamID64()
		}
		return basic.CurrentLiveUniqueID(basic.FlowDeps{PS: params.Dispatch.PlatformSvc}, platformKey)
	})
	params.Dispatch.BasicSvc.StartGameStatsProcessMonitor()
	steam.StartSteamAppListMonitor()

	ctx := wailsApp.Context()
	go func() {
		defer crashlog.Capture()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		last := ""
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			current := platform.CurrentWindowsAccentColor()
			if current != "" && current != last {
				last = current
				_ = wailsApp.Event.Emit(platform.WindowsAccentChangedEvent, current)
			}
		}
	}()

	err = wailsApp.Run()
	if err != nil {
		slog.Error("app run", "err", err)
	}
}

func argvWithoutExecutable(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	return append([]string(nil), args[1:]...)
}

func handleForwardedCLI(app *application.App, disp *Dispatch, argv []string) {
	if app == nil || disp == nil {
		return
	}
	idx, idxErr := cli.LoadPlatformIndex()
	idxPtr := idx
	if idxErr != nil {
		idxPtr = nil
	}
	p, parseErr := cli.Parse(argv, idxPtr)
	if parseErr != nil {
		application.InvokeAsync(func() {
			EmitToast("error", "CLI", parseErr.Error(), 0)
		})
		return
	}
	application.InvokeAsync(func() {
		dispatchCLIInGUI(app, p, disp)
	})
}

func registerNotificationResponseHandler(app *application.App, win *application.WebviewWindow, notifier *notifications.NotificationService) {
	if notifier == nil {
		return
	}
	notifier.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error != nil {
			if app != nil {
				app.Logger.Warn("notification response", "error", result.Error)
			}
			return
		}
		application.InvokeAsync(func() {
			if win != nil {
				win.Show().Focus()
			}
		})
	})
}

func dispatchCLIInGUI(app *application.App, p cli.Parsed, disp *Dispatch) {
	if p.StartInTray {
		application.InvokeSync(func() {
			w := app.Window.Current()
			if w != nil {
				_ = w.Hide()
			}
		})
	}
	switch p.Kind {
	case cli.KindSwapSteam:
		if err := disp.SteamSvc.SwapToSteamAccount(p.SteamID64, p.PersonaState, p.PassthroughLaunchArgs); err != nil {
			EmitToast("error", "Steam", err.Error(), 0)
			return
		}
		if err := disp.LaunchAfterSwap(p); err != nil {
			EmitToast("error", "i18n:Button_Launch", err.Error(), 0)
		}
	case cli.KindSwapBasic:
		if err := basic.SwapTo(basic.FlowDeps{PS: disp.PlatformSvc}, p.PlatformKey, p.UniqueID, p.PassthroughLaunchArgs); err != nil {
			EmitToast("error", "i18n:CLI_Swap", err.Error(), 0)
			return
		}
		if err := disp.LaunchAfterSwap(p); err != nil {
			EmitToast("error", "i18n:Button_Launch", err.Error(), 0)
		}
	case cli.KindLogout:
		if err := disp.RunLogout(p); err != nil {
			EmitToast("error", "i18n:CLI_Logout", err.Error(), 0)
		}
	case cli.KindOpenPage:
		application.InvokeSync(func() {
			w := app.Window.Current()
			if w != nil {
				w.Show().Focus()
			}
		})
		j := p.RouteJSONForOpenPage()
		if j != "" {
			app.Event.Emit("navigate", j)
		}
	default:
		application.InvokeSync(func() {
			w := app.Window.Current()
			if w != nil {
				w.Show().Focus()
			}
		})
		_ = app.Event.Emit("navigate", `{"page":"home"}`)
	}
}

func syncProtocolRegistration() {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return
	}
	s, err := platform.LoadAppSettings(exeDir)
	if err != nil || !s.ProtocolEnabled {
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	_ = winutil.RegisterProtocol(filepath.Clean(self))
}
