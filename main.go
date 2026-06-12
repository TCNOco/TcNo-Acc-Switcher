package main

import (
	"embed"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"TcNo-Acc-Switcher/internal/app"
	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/discordrpc"
	"TcNo-Acc-Switcher/internal/ipc"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/shortcuts"
	"TcNo-Acc-Switcher/internal/stability"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIconPNG []byte

//go:embed updater-key.pub
var updaterPublicKey []byte

var (
	platformSvc = &platform.PlatformService{}
	basicSvc    = basic.NewBasicService(platformSvc)
	steamSvc    = steam.NewSteamService()
	discordRPC  = discordrpc.NewManager()

	crashSubmitted bool
)

func init() {
	winutil.SetEmbeddedFrontendFS(assets)

	application.RegisterEvent[string]("navigate")

	application.RegisterEvent[string]("time")
	application.RegisterEvent[app.ToastPayload]("toast")
	application.RegisterEvent[stability.StabilityPromptPayload]("stability-prompt")
	application.RegisterEvent[steam.AccountPatch](steam.AccountUpdatedEvent)
	application.RegisterEvent[basic.AccountImagePatch](basic.AccountImageUpdatedEvent)
	application.RegisterEvent[basic.GameStatsUpdatedPatch](basic.GameStatsUpdatedEvent)
	application.RegisterEvent[string](platform.ActionBarStatusEvent)
	application.RegisterEvent[shortcuts.ListPayload](shortcuts.UpdatedEvent)
	application.RegisterEvent[[]string](shortcuts.FilesDroppedEvent)
	application.RegisterEvent[platform.UpdateAvailablePayload](platform.AppUpdateAvailableEvent)
	application.RegisterEvent[bool](platform.UpdateCheckFailedEvent)
	application.RegisterEvent[platform.PlatformsJSONUpdatePayload](platform.PlatformsJSONUpdateFoundEvent)
	application.RegisterEvent[platform.PlatformsJSONUpdatePayload](platform.PlatformsJSONUpdatedEvent)
	application.RegisterEvent[platform.UserDataMoveProgressPayload](platform.UserDataMoveProgressEvent)

	platform.SetSteamLaunchHooks(steam.SaveFolderFromConfirmedExe, steam.ResolveSteamExePath)
	platform.SetSteamReset(steam.ResetToDefaults)
	platform.SetDiscordPresenceRefreshHook(discordRPC.RefreshAsync)
	platform.SetPlatformLaunchers(func() error { return steam.LaunchSteamOnly(nil) }, func(platformKey string) error {
		return basic.LaunchBasic(basic.FlowDeps{PS: platformSvc}, platformKey, nil)
	})
	platform.SetPlatformLaunchAs(func(forceAdmin bool) error { return steam.LaunchSteamOnlyAs(forceAdmin, nil) }, func(platformKey string, forceAdmin bool) error {
		return basic.LaunchBasicAs(basic.FlowDeps{PS: platformSvc}, platformKey, forceAdmin, nil)
	})
	app.RegisterStartupAccountCounts()
}

func main() {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "exe dir:", err)
		os.Exit(1)
	}
	if err := platform.InitDataPaths(exeDir); err != nil {
		fmt.Fprintln(os.Stderr, "init data paths:", err)
		os.Exit(1)
	}

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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))

	startupSettings, _ := loadStartupSettings()
	syncOfflineModeFromSettings(startupSettings)

	if crashlog.HasPending() && !startupSettings.OfflineMode && startupSettings.CrashReportAutoSubmit {
		crashSubmitted = crashlog.SubmitPending()
	}
	defer crashlog.Capture()

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

	releaseSingleton, running, err := winutil.TryAcquireSingleton()
	if err != nil {
		fmt.Fprintln(os.Stderr, "singleton:", err)
		os.Exit(1)
	}
	if running {
		if ferr := ipc.ForwardArgs(os.Args[1:]); ferr != nil {
			fmt.Fprintln(os.Stderr, "another instance is running; IPC forward failed:", ferr)
			os.Exit(1)
		}
		os.Exit(0)
	}
	defer releaseSingleton()
	winutil.RegisterSingletonReleaser(releaseSingleton)

	platform.RunUserDataMoveCleanup(exeDir, parsed.UserDataMoveFrom, parsed.UserDataMoveTo)

	syncWindowsStartupFromSettings(startupSettings)

	if parsed.NeedsHeadlessMutex() {
		winutil.AttachParentConsole()
		if herr := disp.RunHeadless(parsed); herr != nil {
			fmt.Fprintln(os.Stderr, herr)
			os.Exit(1)
		}
		os.Exit(0)
	}

	app.RunGUI(app.RunGUIParams{
		Parsed:           parsed,
		GuiSettings:      startupSettings,
		Services:         serviceList(),
		Dispatch:         disp,
		DiscordRPC:       discordRPC,
		CrashSubmitted:   crashSubmitted,
		StartupToast:     parsed.StartupToast,
		EmbeddedAssets:   assets,
		TrayIconPNG:      trayIconPNG,
		UpdaterPublicKey: updaterPublicKey,
	})
}

func serviceList() []application.Service {
	return []application.Service{
		application.NewService(&FilesystemService{}),
		application.NewService(platformSvc),
		application.NewService(steamSvc),
		application.NewService(basicSvc),
		application.NewService(shortcuts.NewService(platformSvc)),
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

func syncWindowsStartupFromSettings(s platform.AppSettings) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	if err := winutil.SyncRunAtStartupTray(filepath.Clean(self), s.StartTrayWithWindows); err != nil {
		log.Printf("windows startup tray sync: %v", err)
	}
}
