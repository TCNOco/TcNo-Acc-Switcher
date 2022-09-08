
using System.Numerics;
using Microsoft.UI;
using Windows.Graphics;
using Microsoft.UI.Windowing;
using Color = Windows.UI.Color;
using Colors = Microsoft.UI.Colors;

namespace TcNo_Acc_Switcher
{
    public partial class App : Application
    {
        public App()
        {
            InitializeComponent();
            Microsoft.Maui.Handlers.WindowHandler.Mapper.AppendToMapping(nameof(IWindow), (handler, view) =>
            {
#if WINDOWS
                var mauiWindow = handler.VirtualView;
                var nativeWindow = handler.PlatformView;
                nativeWindow.Activate();
                var windowHandle = WinRT.Interop.WindowNative.GetWindowHandle(nativeWindow);
                var windowId = Win32Interop.GetWindowIdFromWindow(windowHandle);
                var appWindow = AppWindow.GetFromWindowId(windowId);
                ////appWindow.Resize(new SizeInt32(WindowWidth, WindowHeight));
                appWindow.TitleBar.ExtendsContentIntoTitleBar = true;
                //appWindow.TitleBar.ForegroundColor = Colors.Blue;
                //appWindow.TitleBar.BackgroundColor = Color.FromArgb(0,0,0,0);
                //appWindow.TitleBar.ButtonBackgroundColor = Colors.Purple;
                //nativeWindow.Content.Scale = new Vector3(1,1,1);
                ////nativeWindow.Content.Translation = new Vector3(1, -30, 1);
                //nativeWindow.SetTitleBar();
#endif
            });

            MainPage = new MainPage();

        }
    }
}