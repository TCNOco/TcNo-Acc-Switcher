using System;
using System.Collections.Generic;
using System.Text;
using Microsoft.JSInterop;
using Microsoft.UI;
using Microsoft.UI.Windowing;
using PInvoke;
using TcNo_Acc_Switcher_Globals;
using WinRT.Interop;
using static PInvoke.User32;

namespace TcNo_Acc_Switcher
{
    public static class WindowActions
    {
        public static AppWindow WinUiAppWindow;
        public static IntPtr HWnd;

        public static void OnWindowCreated(Microsoft.UI.Xaml.Window window)
        {
            window.ExtendsContentIntoTitleBar = false;
            var nativeWindowHandle = WindowNative.GetWindowHandle(window);
            var win32WindowsId = Win32Interop.GetWindowIdFromWindow(nativeWindowHandle);
            WinUiAppWindow = AppWindow.GetFromWindowId(win32WindowsId);
            if (WinUiAppWindow.Presenter is OverlappedPresenter p)
            {
                p.SetBorderAndTitleBar(false, false);
            }

            HWnd = WindowNative.GetWindowHandle(window);
            var style = GetWindowLong(HWnd, WindowLongIndexFlags.GWL_STYLE);
            style &= ~(int)(SetWindowLongFlags.WS_CAPTION |
                            SetWindowLongFlags.WS_SYSMENU |
                            SetWindowLongFlags.WS_THICKFRAME |
                            SetWindowLongFlags.WS_MAXIMIZEBOX |
                            SetWindowLongFlags.WS_MINIMIZEBOX);
            SetWindowLong(WindowNative.GetWindowHandle(window), WindowLongIndexFlags.GWL_STYLE, (SetWindowLongFlags)style);
            SetWindowPos(HWnd,
                new IntPtr(0),
                0, 0,
                0, 0,
                SetWindowPosFlags.SWP_NOMOVE |
                SetWindowPosFlags.SWP_NOSIZE |
                SetWindowPosFlags.SWP_FRAMECHANGED);

            //// TODO: Center Window if set to start centered
            //CenterWindow(winUiAppWindow);
        }

        enum WParams
        {
            ScMaximize = 0xF030,
            ScMinimize = 0xF020,
            ScRestore = 0xF120
        }

        // See possible wParams: https://docs.microsoft.com/en-us/windows/win32/menurc/wm-syscommand
        public static void Maximize() => SendMessage(HWnd, WindowMessage.WM_SYSCOMMAND, (IntPtr)WParams.ScMaximize, (IntPtr)0);
        public static void Minimize() => SendMessage(HWnd, WindowMessage.WM_SYSCOMMAND, (IntPtr)WParams.ScMinimize, (IntPtr)0);
        public static void Restore() => SendMessage(HWnd, WindowMessage.WM_SYSCOMMAND, (IntPtr)WParams.ScRestore, (IntPtr)0);
        public static void Exit() => SendMessage(HWnd, WindowMessage.WM_CLOSE, (IntPtr)0, (IntPtr)0);

        [JSInvokable]
        public static void MouseDownDrag()
        {
            ReleaseCapture();
            Console.WriteLine("MouseDownDrag STarted");
            PostMessage(HWnd, WindowMessage.WM_NCLBUTTONDOWN, (IntPtr)0x2, (IntPtr)0); // HtCaption == 0x2
            Console.WriteLine("MouseDownDrag");
        }

        [JSInvokable]
        public static void MouseResizeDrag(int wParam) => MouseResizeDrag((IntPtr) wParam);
        
        private static void MouseResizeDrag(IntPtr wParam)
        {
            ReleaseCapture();
            Console.WriteLine("MouseResizeDrag STarted");
            PostMessage(HWnd, WindowMessage.WM_NCLBUTTONDOWN, wParam, (IntPtr)0);
            Console.WriteLine("MouseResizeDrag");
        }

        public static void OnResizeBorderMouseDown(SysCommandSize pos)
        {
            WindowActions.MouseResizeDrag((IntPtr)pos);
        }
        
        public static void Hide()
        {
            // This should be changed? Need a way to show this from the tray.
            ShowWindow(HWnd, WindowShowStyle.SW_HIDE);
        }

        public static void HideToTray()
        {
            Hide();
            Globals.StartTrayIfNotRunning();
        }

        /// <summary>
        /// Check the current state of the application in Windows
        /// </summary>
        /// <returns>1: Normal, 2: Minimized, 3: Maximized</returns>
        public static int GetCurrentState()
        {
            var placement = NativeFuncs.GetWindowPlacement(HWnd);
            return placement.showCmd;
        }
    }

    // For draggable regions:
    // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
    // This is not used here, but these values are used in js for MouseResizeDrag.
    // Checking once through everything is better than comparing things multiple times.
    public enum SysCommandSize
    {
        ScSizeHtLeft = 0xA, // 1 + 9
        ScSizeHtRight = 0xB,
        ScSizeHtTop = 0xC,
        ScSizeHtTopLeft = 0xD,
        ScSizeHtTopRight = 0xE,
        ScSizeHtBottom = 0xF,
        ScSizeHtBottomLeft = 0x10,
        ScSizeHtBottomRight = 0x11
    }
}
