package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/cli"
	"TcNo-Acc-Switcher/internal/ipc"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/shortcuts"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/tray"
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

//go:embed build/appicon.png
var trayIconPNG []byte

func init() {
	winutil.SetEmbeddedFrontendFS(assets)

	application.RegisterEvent[string]("navigate")

	application.RegisterEvent[string]("time")
	application.RegisterEvent[ToastPayload](toastEventName)
	application.RegisterEvent[steam.AccountPatch](steam.AccountUpdatedEvent)
	application.RegisterEvent[basic.AccountImagePatch](basic.AccountImageUpdatedEvent)
	application.RegisterEvent[string](platform.ActionBarStatusEvent)
	application.RegisterEvent[shortcuts.ListPayload](shortcuts.UpdatedEvent)
	application.RegisterEvent[[]string](shortcuts.FilesDroppedEvent)
	application.RegisterEvent[bool](platform.AppUpdateAvailableEvent)
	application.RegisterEvent[bool](platform.UpdateCheckFailedEvent)

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

	if parsed.IsListCommand() {
		winutil.AttachParentConsole()
		if err := runListCLI(parsed, idx); err != nil {
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

	syncOfflineModeFromSettings()
	syncWindowsStartupFromSettings()

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

type cliListAccountRow struct {
	UniqueID     string `json:"uniqueId"`
	DisplayName  string `json:"displayName"`
	LastLoggedIn string `json:"lastLoggedIn,omitempty"`
}

type cliListPlatformRow struct {
	Code string `json:"code"` // first Identifiers entry (lowercase), for +<code>: swap CLI
	Name string `json:"name"` // canonical platform name from Platforms.json
}

func runListCLI(p cli.Parsed, idx *cli.PlatformIndex) error {
	switch p.Kind {
	case cli.KindListPlatforms:
		if idx == nil {
			return fmt.Errorf("platforms file not loaded")
		}
		rows := make([]cliListPlatformRow, 0, len(idx.OrderedNames))
		for _, name := range idx.OrderedNames {
			code := cli.ShortTokenForPlatform(idx, name)
			if code == "" {
				code = "?"
			}
			rows = append(rows, cliListPlatformRow{Code: code, Name: name})
		}
		if p.OutputJSON {
			b, err := json.Marshal(struct {
				Platforms []cliListPlatformRow `json:"platforms"`
			}{Platforms: rows})
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "code:\tplatform name:\n")
		for _, row := range rows {
			fmt.Fprintf(tw, "%s\t%s\n", row.Code, row.Name)
		}
		_ = tw.Flush()
		return nil

	case cli.KindListAccounts:
		var platNames []string
		if strings.TrimSpace(p.ListAccountsPlatform) != "" {
			platNames = []string{p.ListAccountsPlatform}
		} else {
			if idx == nil {
				return fmt.Errorf("platforms file not loaded")
			}
			platNames = append([]string(nil), idx.OrderedNames...)
		}

		if p.OutputJSON {
			if len(platNames) == 1 {
				rows, err := cliAccountRowsForPlatform(platNames[0])
				if err != nil {
					return err
				}
				b, err := json.Marshal(struct {
					Platform string              `json:"platform"`
					Accounts []cliListAccountRow `json:"accounts"`
				}{Platform: platNames[0], Accounts: rows})
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}
			type platBlock struct {
				Platform string              `json:"platform"`
				Accounts []cliListAccountRow `json:"accounts"`
			}
			blocks := make([]platBlock, 0, len(platNames))
			for _, pk := range platNames {
				rows, err := cliAccountRowsForPlatform(pk)
				if err != nil {
					return fmt.Errorf("%s: %w", pk, err)
				}
				if len(rows) == 0 {
					continue
				}
				blocks = append(blocks, platBlock{Platform: pk, Accounts: rows})
			}
			b, err := json.Marshal(struct {
				Platforms []platBlock `json:"platforms"`
			}{Platforms: blocks})
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}

		for _, pk := range platNames {
			rows, err := cliAccountRowsForPlatform(pk)
			if err != nil {
				return fmt.Errorf("%s: %w", pk, err)
			}
			if len(rows) == 0 {
				continue
			}
			fmt.Printf("%s:\n", pk)
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "  ID\tname\tlast login\n")
			for _, r := range rows {
				last := r.LastLoggedIn
				if last == "" {
					last = "-"
				}
				fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.UniqueID, r.DisplayName, last)
			}
			_ = tw.Flush()
		}
		return nil

	default:
		return fmt.Errorf("internal: not a list command")
	}
}

func cliAccountRowsForPlatform(platformKey string) ([]cliListAccountRow, error) {
	if strings.EqualFold(strings.TrimSpace(platformKey), steam.PlatformKey) {
		accs, err := steamSvc.GetSteamAccounts()
		if err != nil {
			return nil, err
		}
		out := make([]cliListAccountRow, 0, len(accs))
		for _, a := range accs {
			out = append(out, cliListAccountRow{
				UniqueID:     a.SteamID64,
				DisplayName:  strings.TrimSpace(a.DisplayName),
				LastLoggedIn: strings.TrimSpace(a.LastLogin),
			})
		}
		return out, nil
	}
	accs, err := basicSvc.GetAccounts(platformKey)
	if err != nil {
		return nil, err
	}
	out := make([]cliListAccountRow, 0, len(accs))
	for _, a := range accs {
		out = append(out, cliListAccountRow{
			UniqueID:     a.UniqueID,
			DisplayName:  strings.TrimSpace(a.DisplayName),
			LastLoggedIn: strings.TrimSpace(a.LastUsed),
		})
	}
	return out, nil
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

	var guiSettings platform.AppSettings
	if d, err := platform.ResolveExeDir(); err == nil {
		if s, err := platform.LoadAppSettings(d); err == nil {
			guiSettings = s
		}
	}
	if err := stats.IncrementLaunchCount(); err != nil {
		log.Printf("stats launch count: %v", err)
	}
	go stats.MustTryUploadDaily(guiSettings.StatsEnabled, guiSettings.StatsShare, guiSettings.OfflineMode)

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

	winOpts := application.WebviewWindowOptions{
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
	win := app.Window.NewWithOptions(winOpts)
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		app.Event.Emit(shortcuts.FilesDroppedEvent, files)
	})

	trayMgr := tray.NewManager(app, win, tray.Deps{
		SwapBasic: func(platformKey, uniqueID string) error {
			return basicSvc.SwapToAccount(platformKey, uniqueID, nil)
		},
		SwapSteam: func(steamID64 string, personaState int) error {
			return steamSvc.SwapToSteamAccount(steamID64, personaState, nil)
		},
	})
	trayMgr.RegisterCloseHook()
	tray.SetMenuRefresh(trayMgr.RefreshMenu)
	trayMgr.Start(trayIconPNG)

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

func syncWindowsStartupFromSettings() {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return
	}
	s, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	if err := winutil.SyncRunAtStartupTray(filepath.Clean(self), s.StartTrayWithWindows); err != nil {
		log.Printf("windows startup tray sync: %v", err)
	}
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
		showAndFocusMainWindow(app)
		j := p.RouteJSONForOpenPage()
		if j != "" {
			app.Event.Emit("navigate", j)
		}
	default:
		// Empty argv from second-instance launch means "open existing GUI".
		// This also restores the app from tray/hidden state.
		showAndFocusMainWindow(app)
		_ = app.Event.Emit("navigate", `{"page":"home"}`)
	}
}

func showAndFocusMainWindow(app *application.App) {
	if app == nil {
		return
	}
	application.InvokeSync(func() {
		w := app.Window.Current()
		if w == nil {
			return
		}
		w.Show().Focus()
	})
}
