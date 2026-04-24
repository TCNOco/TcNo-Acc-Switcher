package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/ipc"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/shortcuts"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Shared service instances so BasicService uses the same PlatformService as Wails bindings.
var (
	platformSvc = &platform.PlatformService{}
	basicSvc    = basic.NewBasicService(platformSvc)
	steamSvc    = steam.NewSteamService()
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	winutil.SetEmbeddedFrontendFS(assets)

	application.RegisterEvent[string]("navigate")

	application.RegisterEvent[string]("time")
	application.RegisterEvent[ToastPayload](toastEventName)
	application.RegisterEvent[steam.AccountPatch](steam.AccountUpdatedEvent)
	application.RegisterEvent[string](platform.ActionBarStatusEvent)
	application.RegisterEvent[shortcuts.ListPayload](shortcuts.UpdatedEvent)
	application.RegisterEvent[[]string](shortcuts.FilesDroppedEvent)

	platform.SetSteamLaunchHooks(steam.SaveFolderFromConfirmedExe, steam.ResolveSteamExePath)
	platform.SetSteamReset(steam.ResetToDefaults)
	platform.SetPlatformLaunchers(func() error { return steam.LaunchSteamOnly(nil) }, func(platformKey string) error {
		return basic.LaunchBasic(basic.FlowDeps{PS: platformSvc}, platformKey, nil)
	})
	platform.SetPlatformLaunchAs(func(forceAdmin bool) error { return steam.LaunchSteamOnlyAs(forceAdmin, nil) }, func(platformKey string, forceAdmin bool) error {
		return basic.LaunchBasicAs(basic.FlowDeps{PS: platformSvc}, platformKey, forceAdmin, nil)
	})
}

func main() {
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

	if parsed.Kind == cli.KindHelp || parsed.Help {
		fmt.Print(cli.HelpText())
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

	syncOfflineModeFromSettings()

	if parsed.NeedsHeadlessMutex() {
		winutil.AttachParentConsole()
		if herr := runHeadless(parsed); herr != nil {
			fmt.Fprintln(os.Stderr, herr)
			os.Exit(1)
		}
		os.Exit(0)
	}

	runGUI(parsed)
}

func runHeadless(p cli.Parsed) error {
	switch p.Kind {
	case cli.KindSwapSteam:
		if err := steamSvc.SwapToSteamAccount(p.SteamID64, p.PersonaState, p.PassthroughLaunchArgs); err != nil {
			return err
		}
		return launchAfterSwapCLI(p)
	case cli.KindSwapBasic:
		if err := basic.SwapTo(basic.FlowDeps{PS: platformSvc}, p.PlatformKey, p.UniqueID, p.PassthroughLaunchArgs); err != nil {
			return err
		}
		return launchAfterSwapCLI(p)
	case cli.KindLogout:
		return runLogoutCLI(p)
	default:
		return nil
	}
}

// launchAfterSwapCLI runs --run-appid (Steam) or --run-shortcut after a successful swap.
func launchAfterSwapCLI(p cli.Parsed) error {
	if strings.TrimSpace(p.RunAppID) != "" {
		url := "steam://rungameid/" + strings.TrimSpace(p.RunAppID)
		return winutil.Start("cmd.exe", []string{"/c", "start", "", url}, winutil.StartOpts{})
	}
	fn := strings.TrimSpace(p.RunShortcutFile)
	pk := strings.TrimSpace(p.PlatformKey)
	if fn != "" && pk != "" {
		return shortcuts.RunShortcut(pk, fn, false)
	}
	return nil
}

func runLogoutCLI(p cli.Parsed) error {
	plat := strings.TrimSpace(p.LogoutPlatform)
	if plat == "" || strings.EqualFold(plat, "Steam") {
		return steamSvc.SteamAddNew()
	}
	return basic.ClearCurrentLogin(basic.FlowDeps{PS: platformSvc}, plat)
}

func runGUI(parsed cli.Parsed) {
	if parsed.Kind == cli.KindOpenPage {
		platform.SetStartupNavigateHint(parsed.RouteJSONForOpenPage())
	}

	syncProtocolRegistration()

	app := application.New(application.Options{
		Name:        "TcNo Account Switcher",
		Description: "A Superfast open-source account switcher",
		Services: []application.Service{
			application.NewService(&GreetService{}),
			application.NewService(&FilesystemService{}),
			application.NewService(platformSvc),
			application.NewService(steamSvc),
			application.NewService(basicSvc),
			application.NewService(shortcuts.NewService(platformSvc)),
		},
		Assets: application.AssetOptions{
			Handler: newCompositeAssetHandler(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	if err := ipc.StartGUIServer(func(argv []string) {
		handleForwardedCLI(app, argv)
	}); err != nil {
		log.Printf("ipc server: %v", err)
	}

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "TcNo Account Switcher",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
		Frameless:        true,
		EnableFileDrop:   true,
	})
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		app.Event.Emit(shortcuts.FilesDroppedEvent, files)
	})

	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	err := app.Run()
	if err != nil {
		log.Fatal(err)
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

func syncOfflineModeFromSettings() {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return
	}
	s, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return
	}
	appclient.SetOfflineMode(s.OfflineMode)
}

func handleForwardedCLI(app *application.App, argv []string) {
	idx, err := cli.LoadPlatformIndex()
	idxPtr := idx
	if err != nil {
		idxPtr = nil
	}
	p, err := cli.Parse(argv, idxPtr)
	if err != nil {
		application.InvokeAsync(func() {
			EmitToast("error", "CLI", err.Error(), 0)
		})
		return
	}

	application.InvokeAsync(func() {
		dispatchCLIInGUI(app, p)
	})
}

func dispatchCLIInGUI(app *application.App, p cli.Parsed) {
	switch p.Kind {
	case cli.KindSwapSteam:
		if err := steamSvc.SwapToSteamAccount(p.SteamID64, p.PersonaState, p.PassthroughLaunchArgs); err != nil {
			EmitToast("error", "Steam", err.Error(), 0)
			return
		}
		if err := launchAfterSwapCLI(p); err != nil {
			EmitToast("error", "Launch", err.Error(), 0)
		}
	case cli.KindSwapBasic:
		if err := basic.SwapTo(basic.FlowDeps{PS: platformSvc}, p.PlatformKey, p.UniqueID, p.PassthroughLaunchArgs); err != nil {
			EmitToast("error", "Swap", err.Error(), 0)
			return
		}
		if err := launchAfterSwapCLI(p); err != nil {
			EmitToast("error", "Launch", err.Error(), 0)
		}
	case cli.KindLogout:
		if err := runLogoutCLI(p); err != nil {
			EmitToast("error", "Logout", err.Error(), 0)
		}
	case cli.KindOpenPage:
		j := p.RouteJSONForOpenPage()
		if j != "" {
			app.Event.Emit("navigate", j)
		}
	default:
		// empty argv from second-instance launch — ignore
	}
}
