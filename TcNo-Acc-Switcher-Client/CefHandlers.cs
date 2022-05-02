using System;
using System.Diagnostics;
using System.Windows.Forms;
using CefSharp;

namespace TcNo_Acc_Switcher_Client
{
    public class CefKeyboardHandler : IKeyboardHandler
    {
        public bool OnKeyEvent(IWebBrowser browserControl, IBrowser browser, KeyType type, int windowsKeyCode,
            int nativeKeyCode, CefEventFlags modifiers, bool isSystemKey)
        {
            if (type != KeyType.KeyUp || !Enum.IsDefined(typeof(Keys), windowsKeyCode)) return false;

            var key = (Keys)windowsKeyCode;

            if (key == Keys.F12)
                browser.ShowDevTools();
            else if (key == Keys.F5 && modifiers == CefEventFlags.ControlDown) // Ctrl+F5 - Cache reload
                browser.Reload(true);
            else if (key == Keys.F5) // F5 - Reload
                browser.Reload();
            else if (key == Keys.I && modifiers == (CefEventFlags.ControlDown | CefEventFlags.ShiftDown)) // Ctrl+Shift+I
                browser.ShowDevTools();
            else if (key == Keys.F1) // F1 - Opens Wiki page
                Process.Start(new ProcessStartInfo { FileName = "https://github.com/TcNobo/TcNo-Acc-Switcher/wiki", UseShellExecute = true });

            return false;
        }
        public bool OnPreKeyEvent(IWebBrowser browserControl, IBrowser browser, KeyType type, int windowsKeyCode, int nativeKeyCode, CefEventFlags modifiers, bool isSystemKey, ref bool isKeyboardShortcut)
        {
            return false;
        }
    }
    public class CefMenuHandler : IContextMenuHandler
    {
        public void OnBeforeContextMenu(IWebBrowser browserControl, IBrowser browser, IFrame frame, IContextMenuParams parameters, IMenuModel model)
        {
            model.Clear();
        }

        public bool OnContextMenuCommand(IWebBrowser browserControl, IBrowser browser, IFrame frame, IContextMenuParams parameters, CefMenuCommand commandId, CefEventFlags eventFlags)
        {

            return false;
        }

        public void OnContextMenuDismissed(IWebBrowser browserControl, IBrowser browser, IFrame frame)
        {

        }

        public bool RunContextMenu(IWebBrowser browserControl, IBrowser browser, IFrame frame, IContextMenuParams parameters, IMenuModel model, IRunContextMenuCallback callback)
        {
            return false;
        }
    }
}
