using Windows.Graphics;
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
                        style &= ~(int)(User32.SetWindowLongFlags.WS_CAPTION |
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

            builder.Services.AddSingleton<WeatherForecastService>();

            return builder.Build();
        }
    }

    //private void CenterWindow(AppWindow winUiAppWindow)
    //{
    //    var winSize = winUiAppWindow.Size;
    //    winUiAppWindow.MoveAndResize(new RectInt32(1920 / 2 - winSize.Width / 2, 1080 / 2 - winSize.Height / 2, winSize.Width, winSize.Height));
    //}
}