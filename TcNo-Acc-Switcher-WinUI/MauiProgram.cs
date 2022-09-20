using Windows.Graphics;
using Microsoft.AspNetCore.Components.WebView.Maui;
using Microsoft.Maui.LifecycleEvents;
using Microsoft.UI.Windowing;
using Microsoft.UI;
using PInvoke;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State;
using TcNo_Acc_Switcher.State.Interfaces;
using WinRT.Interop;

namespace TcNo_Acc_Switcher
{
    public static class MauiProgram
    {
        public static MauiApp CreateMauiApp()
        {
            #if DEBUG
            //if (!NativeFuncs.AttachToConsole(-1)) // Attach to a parent process console (ATTACH_PARENT_PROCESS)
                _ = NativeFuncs.AllocConsole();
            #endif
            
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
                    wndLifeCycleBuilder.OnWindowCreated(WindowActions.OnWindowCreated);
                });
            });
#endif
            builder.Services.AddMauiBlazorWebView();
#if DEBUG
            builder.Services.AddBlazorWebViewDeveloperTools();
#endif

            // Proper singletons. This is the correct load order.
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