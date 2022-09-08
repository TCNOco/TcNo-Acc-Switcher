using Windows.Graphics;
using Microsoft.AspNetCore.Components.WebView.Maui;
using Microsoft.Maui.LifecycleEvents;
using Microsoft.UI.Windowing;
using Microsoft.UI;
using PInvoke;
using TcNo_Acc_Switcher.State;
using TcNo_Acc_Switcher.State.Interfaces;
using WinRT.Interop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher
{
    public static class MauiProgram
    {
        public static MauiApp CreateMauiApp()
        {
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;

            var builder = MauiApp.CreateBuilder();
            builder
                .UseMauiApp<App>()
                .ConfigureFonts(fonts =>
                {
                    fonts.AddFont("OpenSans-Regular.ttf", "OpenSansRegular");
                });

#if WINDOWS
            // Remove the frame
            builder.ConfigureLifecycleEvents(events =>
            {
                events.AddWindows(wndLifeCycleBuilder =>
                {
                    wndLifeCycleBuilder.OnWindowCreated(window =>
                    {
                        window.ExtendsContentIntoTitleBar = false;
                        var nativeWindowHandle = WindowNative.GetWindowHandle(window);
                        var win32WindowsId = Win32Interop.GetWindowIdFromWindow(nativeWindowHandle);
                        var winUiAppWindow = AppWindow.GetFromWindowId(win32WindowsId);
                        if (winUiAppWindow.Presenter is OverlappedPresenter p)
                        {
                            p.SetBorderAndTitleBar(false, false);
                        }

                        var hwnd = WindowNative.GetWindowHandle(window);
                        var style = User32.GetWindowLong(hwnd, User32.WindowLongIndexFlags.GWL_STYLE);
                        style &= ~(int) (User32.SetWindowLongFlags.WS_CAPTION |
                                         User32.SetWindowLongFlags.WS_SYSMENU |
                                         User32.SetWindowLongFlags.WS_THICKFRAME |
                                         User32.SetWindowLongFlags.WS_MAXIMIZEBOX |
                                         User32.SetWindowLongFlags.WS_MINIMIZEBOX);
                        User32.SetWindowLong(WindowNative.GetWindowHandle(window), User32.WindowLongIndexFlags.GWL_STYLE, (User32.SetWindowLongFlags)style);
                        User32.SetWindowPos(hwnd,
                            new IntPtr(0),
                            0, 0,
                            0, 0,
                            User32.SetWindowPosFlags.SWP_NOMOVE |
                            User32.SetWindowPosFlags.SWP_NOSIZE |
                            User32.SetWindowPosFlags.SWP_FRAMECHANGED);

                        //// TODO: Center Window if set to start centered
                        //CenterWindow(winUiAppWindow);
                    });
                });
            });
#endif
            builder.Services.AddMauiBlazorWebView();
#if DEBUG
		builder.Services.AddBlazorWebViewDeveloperTools();
#endif

            builder.Services.AddSingleton<IWindowSettings, WindowSettings>(); // #1 - NONE
            builder.Services.AddSingleton<ILang, Lang>(); // After WindowSettings
            builder.Services.AddSingleton<IToasts, Toasts>(); // After Lang
            builder.Services.AddSingleton<IModals, Modals>(); // After Lang
            builder.Services.AddSingleton<IStatistics, Statistics>(); // After WindowSettings
            builder.Services.AddSingleton<IAppState, AppState>(); // Lang, Modals, Statistics, Toasts, WindowSettings
            builder.Services.AddSingleton<ISharedFunctions, SharedFunctions>(); // AppState, Lang, Modals, Statistics, Toasts (+JsRuntime)

            builder.Services.AddSingleton<IGameStatsRoot, GameStatsRoot>(); // Toasts, WindowSettings
            builder.Services.AddSingleton<IGameStats, GameStats>(); // AppState, GameStatsRoot

            builder.Services.AddSingleton<ISteamSettings, SteamSettings>(); // (No depends)
            builder.Services.AddSingleton<ISteamState, SteamState>(); // AppState, GameStats, Lang, Modals, SharedFunctions, Statistics, SteamSettings, Toasts
            builder.Services.AddSingleton<ISteamFuncs, SteamFuncs>(); // AppState, Lang, Modals, SharedFunctions, Statistics, SteamSettings, SteamState, Toasts, WindowSettings (+JSRuntime)

            builder.Services.AddSingleton<ITemplatedPlatformState, TemplatedPlatformState>(); // AppState, GameStats, Modals, SharedFunctions, Statistics, Toasts
            builder.Services.AddSingleton<ITemplatedPlatformSettings, TemplatedPlatformSettings>(); // Statistics, TemplatedPlatformState
            builder.Services.AddSingleton<ITemplatedPlatformFuncs, TemplatedPlatformFuncs>(); // AppState, Lang, Modals, SharedFunctions, Statistics, TemplatedPlatformState, TemplatedPlatformSettings, Toasts, WindowSettings (+JSRuntime, NavigationManager)

            return builder.Build();
        }
    }

    //private void CenterWindow(AppWindow winUiAppWindow)
    //{
    //    var winSize = winUiAppWindow.Size;
    //    winUiAppWindow.MoveAndResize(new RectInt32(1920 / 2 - winSize.Width / 2, 1080 / 2 - winSize.Height / 2, winSize.Width, winSize.Height));
    //}
}