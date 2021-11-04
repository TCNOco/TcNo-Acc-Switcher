using CefSharp.WinForms;
using System;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Windows.Forms;

namespace TcNo_Acc_Switcher_Client_CEF
{

    class ChromeWidgetMessageInterceptor : NativeWindow
    {
        private readonly ChromiumWebBrowser _browser;
        private Action<Message> _forwardAction;

        private ChromeWidgetMessageInterceptor(ChromiumWebBrowser browser, IntPtr chromeWidgetHostHandle, Action<Message> forwardAction)
        {
            AssignHandle(chromeWidgetHostHandle);

            _browser = browser;
            browser.HandleDestroyed += BrowserHandleDestroyed;

            _forwardAction = forwardAction;
        }

        internal static void SetupLoop(ChromiumWebBrowser browser, Action<Message> forwardAction)
        {
            Task.Factory.StartNew(() =>
            {
                try
                {
                    var foundWidget = false;
                    while (!foundWidget)
                    {
                        browser.Invoke((Action)(() =>
                        {
                            if (ChromeWidgetHandleFinder.TryFindHandle(browser, out var chromeWidgetHostHandle))
                            {
                                foundWidget = true;
                                var chromeWidgetMessageInterceptor = new ChromeWidgetMessageInterceptor(browser, chromeWidgetHostHandle, forwardAction);
                            }
                            else
                                Thread.Sleep(10);
                        }));
                    }
                }
                catch
                {
                    //
                }
            });
        }

        private void BrowserHandleDestroyed(object sender, EventArgs e)
        {
            ReleaseHandle();

            _browser.HandleDestroyed -= BrowserHandleDestroyed;
            _forwardAction = null;
        }

        protected override void WndProc(ref Message m)
        {
            base.WndProc(ref m);
            _forwardAction?.Invoke(m);
        }
    }

    class ChromeWidgetHandleFinder
    {
        private delegate bool EnumWindowProc(IntPtr hwnd, IntPtr lParam);

        [DllImport("user32")]
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool EnumChildWindows(IntPtr window, EnumWindowProc callback, IntPtr lParam);

        private readonly IntPtr _mainHandle;
        private string _seekClassName;
        private IntPtr _descendantFound;

        private ChromeWidgetHandleFinder(IntPtr handle)
        {
            _mainHandle = handle;
        }

        private IntPtr FindDescendantByClassName(string className)
        {
            _descendantFound = IntPtr.Zero;
            _seekClassName = className;

            EnumWindowProc childProc = EnumWindow;
            EnumChildWindows(_mainHandle, childProc, IntPtr.Zero);

            return _descendantFound;
        }

        [DllImport("user32.dll", CharSet = CharSet.Auto)]
        public static extern int GetClassName(IntPtr hWnd, StringBuilder lpClassName, int nMaxCount);

        private bool EnumWindow(IntPtr hWnd, IntPtr lParam)
        {
            var buffer = new StringBuilder(128);
            GetClassName(hWnd, buffer, buffer.Capacity);

            if (buffer.ToString() != _seekClassName) return true;
            _descendantFound = hWnd;
            return false;

        }

        public static bool TryFindHandle(ChromiumWebBrowser browser, out IntPtr chromeWidgetHostHandle)
        {
            var browserHandle = browser.Handle;
            var windowHandleInfo = new ChromeWidgetHandleFinder(browserHandle);
            const string chromeWidgetHostClassName = "Chrome_RenderWidgetHostHWND";
            chromeWidgetHostHandle = windowHandleInfo.FindDescendantByClassName(chromeWidgetHostClassName);
            return chromeWidgetHostHandle != IntPtr.Zero;
        }
    }
}
