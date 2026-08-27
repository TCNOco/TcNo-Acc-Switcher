package main

import (
	"embed"
	"fmt"
	"log"
	"log/slog"
	"os"

	"TcNo-Acc-Switcher/internal/actionlog"
	"TcNo-Acc-Switcher/internal/app"
	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/buildmode"
	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/controllerinput"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/discordrpc"
	"TcNo-Acc-Switcher/internal/gzipfs"
	"TcNo-Acc-Switcher/internal/ipc"
	"TcNo-Acc-Switcher/internal/legacyinstall"
	"TcNo-Acc-Switcher/internal/logredact"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/serverpicker"
	"TcNo-Acc-Switcher/internal/shortcuts"
	"TcNo-Acc-Switcher/internal/stability"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steambrowser"
	"TcNo-Acc-Switcher/internal/steamguard"
	"TcNo-Acc-Switcher/internal/tray"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

//go:embed all:frontend/dist
var assets embed.FS

// The production build stores most of frontend/dist gzipped; the wrapper hides
// that from every consumer, so both the asset server and the platform art
// lookup take this rather than the raw embed.
var frontendAssets = gzipfs.New(assets)

//go:embed build/trayicon.png
var trayIconPNG []byte

//go:embed updater-key.pub
var updaterPublicKey []byte

var (
	platformSvc   = &platform.PlatformService{}
	basicSvc      = basic.NewBasicService(platformSvc)
	steamSvc      = steam.NewSteamService()
	steamGuardSvc = steamguard.NewService()
	// The browser windows draw their session from the vault, so the Steam Guard
	// service is their session source. Its data path is resolved at startup,
	// where a failure can be reported.
	steamBrowserSvc *steambrowser.Service
	controllerSvc   = controllerinput.NewService()
	securitySvc     = security.NewService()
	serverPickerSvc = serverpicker.NewService()
	discordRPC      = discordrpc.NewManager()

	crashSubmitted bool
)

func init() {
	winutil.SetEmbeddedFrontendFS(frontendAssets)

	application.RegisterEvent[string]("navigate")

	application.RegisterEvent[app.ToastPayload]("toast")
	application.RegisterEvent[stability.StabilityPromptPayload]("stability-prompt")
	application.RegisterEvent[string](controllerinput.EventName)
	application.RegisterEvent[steam.AccountPatch](steam.AccountUpdatedEvent)
	application.RegisterEvent[basic.AccountImagePatch](basic.AccountImageUpdatedEvent)
	application.RegisterEvent[basic.GameStatsUpdatedPatch](basic.GameStatsUpdatedEvent)
	application.RegisterEvent[string](platform.ActionBarStatusEvent)
	application.RegisterEvent[shortcuts.ListPayload](shortcuts.UpdatedEvent)
	application.RegisterEvent[shortcuts.FilesDroppedPayload](shortcuts.FilesDroppedEvent)
	application.RegisterEvent[bool](platform.ScreenCoveredEvent)
	application.RegisterEvent[bool](platform.GameRunningEvent)
	application.RegisterEvent[platform.UpdateAvailablePayload](platform.AppUpdateAvailableEvent)
	application.RegisterEvent[bool](platform.UpdateCheckFailedEvent)
	application.RegisterEvent[platform.PlatformsJSONUpdatePayload](platform.PlatformsJSONUpdateFoundEvent)
	application.RegisterEvent[platform.PlatformsJSONUpdatePayload](platform.PlatformsJSONUpdatedEvent)
	application.RegisterEvent[platform.UserDataMoveProgressPayload](platform.UserDataMoveProgressEvent)
	application.RegisterEvent[serverpicker.PingResult](serverpicker.PingEvent)
	application.RegisterEvent[string](serverpicker.PingDoneEvent)

	// Steam Guard enrollment adds accounts to the Steam account store from a
	// package that holds no SteamService; this is how it asks for their avatars.
	steam.RegisterProfileRefreshTrigger(steamSvc.StartSteamProfileRefresh)
	// An account restored from a Steam Guard vault is known only by SteamID64
	// until the vault is asked for its login name.
	steamguard.RegisterAccountNameResolver(steamGuardSvc)
	// Forgetting an account has to take its Steam Guard record with it, or the
	// account list rebuilds a nameless row from the Steam Guard index.
	steamguard.RegisterForgetHandler(steamGuardSvc)

	platform.SetSteamLaunchHooks(steam.SaveFolderFromConfirmedExe, steam.ResolveSteamExePath)
	platform.SetSteamReset(steam.ResetToDefaults)
	platform.SetControllerSupportChangedHook(controllerSvc.SetEnabled)
	platform.SetDiscordPresenceRefreshHook(discordRPC.RefreshAsync)
	platform.SetPlatformLaunchers(func() error { return steam.LaunchSteamOnly(nil) }, func(platformKey string) error {
		return basic.LaunchBasic(basic.FlowDeps{PS: platformSvc}, platformKey, nil)
	})
	platform.SetPlatformLaunchAs(func(forceAdmin bool) error { return steam.LaunchSteamOnlyAs(forceAdmin, nil) }, func(platformKey string, forceAdmin bool) error {
		return basic.LaunchBasicAs(basic.FlowDeps{PS: platformSvc}, platformKey, forceAdmin, nil)
	})
	security.SetStatusChangedHook(func() {
		if !security.AppLocked() {
			basic.SyncAllTrayKnownAccounts()
			steam.SyncTrayKnownAccounts()
		}
		tray.RefreshMenuIfSet()
	})
	app.RegisterStartupAccountCounts()
}

func main() {
	// MUST be the first statement in main. The updater's Restart re-executes
	// this exe as its swap helper; Wails only enters helper mode inside
	// application.New, which is after the singleton check below — the helper
	// would see the still-running parent, forward its args and exit without
	// ever swapping the binary (the app "vanishes" and stays on the old
	// version). In helper mode this call never returns.
	updater.HandleHelperMode()

	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "exe dir:", err)
		os.Exit(1)
	}
	if err := platform.InitDataPaths(exeDir); err != nil {
		fmt.Fprintln(os.Stderr, "init data paths:", err)
		os.Exit(1)
	}
	security.CleanupTransientState()

	idx, idxErr := cli.LoadPlatformIndex()
	idxPtr := idx
	if idxErr != nil {
		log.Printf("cli platforms index: %v", idxErr)
		idxPtr = nil
	}

	parsed, err := cli.Parse(os.Args[1:], idxPtr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	lvl := app.ResolvedLogLevel(parsed)
	logSink, logSinkErr := app.OpenLogSink()
	if logSink != nil {
		defer logSink.Close()
	}
	slog.SetDefault(slog.New(logredact.NewHandler(slog.NewTextHandler(app.LogWriter(logSink), &slog.HandlerOptions{Level: lvl}))))
	if logSinkErr != nil {
		slog.Warn("on-disk log unavailable; logging to stderr only", "err", logSinkErr)
	}
	actionlog.Init()

	startupSettings, _ := loadStartupSettings()
	syncOfflineModeFromSettings(startupSettings)
	stats.SetStatsCollectionEnabled(startupSettings.StatsEnabled)

	if crashlog.HasPending() && !startupSettings.OfflineMode && startupSettings.CrashReportAutoSubmit {
		crashSubmitted = crashlog.SubmitPending()
	}
	defer crashlog.CaptureFatal()

	if parsed.Kind == cli.KindHelp || parsed.Help {
		fmt.Print(cli.HelpText())
		os.Exit(0)
	}

	disp := &app.Dispatch{
		SteamSvc:    steamSvc,
		BasicSvc:    basicSvc,
		PlatformSvc: platformSvc,
	}

	if parsed.IsListCommand() {
		winutil.AttachParentConsole()
		if err := disp.RunList(parsed, idx); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if parsed.Kind == cli.KindCleanLegacyInstall {
		winutil.AttachParentConsole()
		os.Exit(legacyinstall.RunCleanupCommand(exeDir))
	}

	// Before anything can ask whether this process is the helper, and before the
	// breakaway, which must not move a helper that is already outside the tree.
	if parsed.GameModeSwitch {
		steam.MarkSwitchHelper()
	}

	// Before the singleton, because the replacement needs to be the one holding
	// it, and after the CLI commands above, which are short-lived and never
	// close Steam.
	if brokeAway, berr := steam.BreakAwayFromSteamLaunch(); brokeAway {
		os.Exit(0)
	} else if berr != nil {
		slog.Warn("still inside Steam's process tree; closing Steam will close the switcher with it", "err", berr)
	}

	releaseSingleton, running, err := winutil.TryAcquireSingleton()
	if err != nil {
		// No console is attached on the GUI path, so slog is the only way these
		// two reach the user at all: they are the "it just does not start" and
		// "it vanished" reports, and both used to exit without a trace.
		slog.Error("startup aborted: could not acquire the single-instance lock", "err", err)
		os.Exit(1)
	}
	if running {
		if ferr := ipc.ForwardArgs(os.Args[1:]); ferr != nil {
			slog.Error("another instance is running and forwarding the command to it failed", "err", ferr)
			os.Exit(1)
		}
		os.Exit(0)
	}
	// Wrapped so a failed elevation below can swap in the lock it re-took.
	defer func() { releaseSingleton() }()
	winutil.RegisterSingletonReleaser(releaseSingleton)

	// "Always run as admin" hands over to an elevated copy here, after the
	// single-instance check, so a launch that only forwards a command to a
	// running instance does not raise a second UAC prompt to do it.
	if app.ShouldElevateAtStartup(parsed, startupSettings) {
		if err := winutil.RestartElevated(os.Args[1:]); err != nil {
			// Declined at the UAC prompt, or blocked by policy. Carry on
			// unelevated rather than leaving a preference the user cannot reach
			// to turn off. RestartElevated drops the single-instance lock before
			// it asks, so take it back.
			slog.Warn("always run as admin: continuing unelevated", "err", err)
			if again, alreadyRunning, aerr := winutil.TryAcquireSingleton(); aerr == nil && !alreadyRunning {
				releaseSingleton = again
				winutil.RegisterSingletonReleaser(again)
			}
		}
	}

	platform.RunUserDataMoveCleanup(exeDir, parsed.UserDataMoveFrom, parsed.UserDataMoveTo)

	// Before any Steam Guard window is created, so both the modal's sensitive
	// views and the confirmations window see the same answer.
	steamguard.ResolveCapturePolicy(parsed.AllowSteamGuardCapture)

	if parsed.NeedsHeadlessMutex() {
		winutil.AttachParentConsole()
		if herr := disp.RunHeadless(parsed); herr != nil {
			fmt.Fprintln(os.Stderr, herr)
			os.Exit(1)
		}
		os.Exit(0)
	}

	legacyinstall.StartupCleanup(exeDir)

	app.RunGUI(app.RunGUIParams{
		Parsed:           parsed,
		GuiSettings:      startupSettings,
		Services:         serviceList(),
		Dispatch:         disp,
		DiscordRPC:       discordRPC,
		ControllerInput:  controllerSvc,
		LogWriter:        app.LogWriter(logSink),
		CrashSubmitted:   crashSubmitted,
		StartupToast:     parsed.StartupToast,
		EmbeddedAssets:   frontendAssets,
		TrayIconPNG:      trayIconPNG,
		AppIconPNG:       trayIconPNG,
		UpdaterPublicKey: updaterPublicKey,
	})
}

func serviceList() []application.Service {
	// Built here rather than in the var block because the data path can fail, and
	// a failure is worth logging rather than swallowing. Without it the service
	// still registers and reports itself unavailable, so the UI hides the entry
	// points instead of offering something that cannot work.
	browserDataPath, err := paths.SteamBrowserDir()
	if err != nil {
		log.Printf("steam browser data path: %v", err)
	}
	steamBrowserSvc = steambrowser.NewService(steamguard.NewBrowserSessionSource(steamGuardSvc), browserDataPath, buildmode.IsDebugBuild())

	return []application.Service{
		application.NewService(&FilesystemService{}),
		application.NewService(platformSvc),
		application.NewService(steamSvc),
		application.NewService(steamGuardSvc),
		application.NewService(controllerSvc),
		application.NewService(basicSvc),
		application.NewService(securitySvc),
		application.NewService(shortcuts.NewService(platformSvc)),
		application.NewService(steamBrowserSvc),
		application.NewService(serverPickerSvc),
	}
}

func loadStartupSettings() (platform.AppSettings, error) {
	d, err := platform.ResolveExeDir()
	if err != nil {
		return platform.AppSettings{}, err
	}
	return platform.LoadAppSettings(d)
}

func syncOfflineModeFromSettings(s platform.AppSettings) {
	appclient.SetOfflineMode(s.OfflineMode)
}
