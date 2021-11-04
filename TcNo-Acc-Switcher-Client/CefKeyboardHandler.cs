using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Forms;
using CefSharp;

namespace TcNo_Acc_Switcher_Client
{
    class CefKeyboardHandler : IKeyboardHandler
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

            return false;
        }
        public bool OnPreKeyEvent(IWebBrowser browserControl, IBrowser browser, KeyType type, int windowsKeyCode, int nativeKeyCode, CefEventFlags modifiers, bool isSystemKey, ref bool isKeyboardShortcut)
        {
            return false;
        }
    }
}
