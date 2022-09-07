using Microsoft.AspNetCore.Components.WebView.Maui;
using Microsoft.Maui.LifecycleEvents;
using Microsoft.UI.Windowing;
using Microsoft.UI;
using PInvoke;
using TcNo_Acc_Switcher.Data;
using WinRT.Interop;

namespace TcNo_Acc_Switcher
{
    public static class MauiProgram
    {
        public static MauiApp CreateMauiApp()
        {
            var builder = MauiApp.CreateBuilder();
            builder
                .UseMauiApp<App>()
                .ConfigureFonts(fonts =>
                {
                    fonts.AddFont("OpenSans-Regular.ttf", "OpenSansRegular");
                });

            builder.ConfigureLifecycleEvents(lifecycle => {
#if WINDOWS
                //lifecycle
                //    .AddWindows(windows =>
                //        windows.OnNativeMessage((app, args) => {
                //            if (WindowExtensions.Hwnd == IntPtr.Zero)
                //            {
                //                WindowExtensions.Hwnd = args.Hwnd;
                //                WindowExtensions.SetIcon("Platforms/Windows/trayicon.ico");
                //            }
                //        }));

                lifecycle.AddWindows(windows => windows.OnWindowCreated((del) => {
                    del.ExtendsContentIntoTitleBar = true;
                    //del.SetTitleBar(null);
                    //////User32.SetWindowLong(
                    //////    WindowNative.GetWindowHandle(del),
                    //////    User32.WindowLongIndexFlags.GWL_STYLE,
                    //////    (User32.SetWindowLongFlags.WS_VISIBLE));
                    var hwnd = WindowNative.GetWindowHandle(del);
                    var style = User32.GetWindowLong(hwnd, User32.WindowLongIndexFlags.GWL_STYLE);
                    style &= ~(int)User32.SetWindowLongFlags.WS_CAPTION;
                    style = style & ~(int)User32.SetWindowLongFlags.WS_SYSMENU;
                    style = style & ~(int)User32.SetWindowLongFlags.WS_THICKFRAME;
                    style = style & ~(int)User32.SetWindowLongFlags.WS_MINIMIZE;
                    style = style & ~(int)User32.SetWindowLongFlags.WS_MAXIMIZEBOX;
                    style = style & ~(int)User32.SetWindowLongFlags.WS_MAXIMIZE;
                    style = style & ~(int)User32.SetWindowLongFlags.WS_MINIMIZEBOX;

                    var test1 = User32.SetWindowLong(WindowNative.GetWindowHandle(del), User32.WindowLongIndexFlags.GWL_STYLE, (User32.SetWindowLongFlags)style);
                    //var test1 = User32.SetWindowLong(WindowNative.GetWindowHandle(del), User32.WindowLongIndexFlags.GWL_STYLE, (User32.SetWindowLongFlags)0xC00000);

                    var test2 = User32.SetWindowLong(hwnd,
                        User32.WindowLongIndexFlags.GWL_EXSTYLE,
                        (User32.SetWindowLongFlags)(User32.GetWindowLong(hwnd, User32.WindowLongIndexFlags.GWL_EXSTYLE))
                            | User32.SetWindowLongFlags.WS_EX_DLGMODALFRAME);
                    var test3 = User32.SetWindowPos(hwnd, new IntPtr(0), 0, 0, 0, 0, User32.SetWindowPosFlags.SWP_NOMOVE | User32.SetWindowPosFlags.SWP_NOSIZE | User32.SetWindowPosFlags.SWP_FRAMECHANGED);


                    //_ = User32.SetWindowLong(hwnd,
                    //    User32.WindowLongIndexFlags.GWL_STYLE, User32.SetWindowLongFlags.WS_VISIBLE);
                    //_ = User32.SetWindowLong(hwnd,
                    //    User32.WindowLongIndexFlags.GWL_EXSTYLE, User32.SetWindowLongFlags.WS_EX_TOPMOST);

                    //var hwnd = WindowNative.GetWindowHandle(del);
                    //var windowId = Win32Interop.GetWindowIdFromWindow(hwnd);
                    //var appWindow = AppWindow.GetFromWindowId(windowId);
                    //appWindow.SetPresenter(AppWindowPresenterKind.FullScreen);
                }));
#endif
            });


            builder.Services.AddMauiBlazorWebView();
#if DEBUG
		builder.Services.AddBlazorWebViewDeveloperTools();
#endif

            builder.Services.AddSingleton<WeatherForecastService>();

            return builder.Build();
        }
    }
}