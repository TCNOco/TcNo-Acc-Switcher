using CefSharp;
using CefSharp.WinForms;
using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Web;
using System.Windows;
using System.Windows.Forms;
using TcNo_Acc_Switcher_Globals_Framework;

namespace TcNo_Acc_Switcher_Client_CEF
{
    public partial class MainForm : Form
    {
        private IntPtr hwnd;
        public MainForm()
        {
            InitializeComponent();
        }

        public ChromiumWebBrowser CefView;

        // Messages
        const int WM_MOUSEMOVE = 0x0200;
        const int WM_MOUSELEAVE = 0x02A3;
        const int WM_LBUTTONDOWN = 0x0201;
        const int WM_LBUTTONUP = 0x0202;

        [DllImport("user32.dll")]
        public static extern int SendMessage(IntPtr hWnd, Int32 wMsg, bool wParam, Int32 lParam);

        private const int WM_SETREDRAW = 11;

        public static void SuspendDrawing(Control control) => SendMessage(control.Handle, WM_SETREDRAW, false, 0);

        public static void ResumeDrawing(Control control)
        {
            SendMessage(control.Handle, WM_SETREDRAW, true, 0);
            control.Refresh();
        }
        protected override void OnResize(EventArgs e)
        {
            SuspendDrawing(this);
            base.OnResize(e);
            ResumeDrawing(this);
        }

        protected override void OnResizeBegin(EventArgs e)
        {
            SuspendDrawing(this);
            base.OnResizeBegin(e);
        }

        protected override void OnResizeEnd(EventArgs e)
        {
            base.OnResizeEnd(e);
            ResumeDrawing(this);
        }

        protected override void OnClosing(CancelEventArgs e)
        {
            SuspendDrawing(this);
            base.OnClosing(e);
            ResumeDrawing(this);
        }
        private void Main_Load(object sender, EventArgs e)
        {
            InitializeChromium();
            hwnd = this.Handle;
            ReleaseCapture();
            SendMessage(Handle, 161, 10, 0);
        }

        private void InitializeChromium()
        {
            Globals.DebugWriteLine(@"[Func:(Client-CEF)MainWindow.xaml.cs.InitializeChromium]");
            CefSettings settings = new CefSettings()
            {
                CachePath = Globals.PathJoin(Globals.UserDataFolder, "CEF\\Cache"),
                UserAgent = "TcNo-CEF 1.0",
                UserDataPath = Globals.PathJoin(Globals.UserDataFolder, "CEF\\Data"),
                WindowlessRenderingEnabled = false
            };
            settings.CefCommandLineArgs.Add("-off-screen-rendering-enabled", "0");
            settings.CefCommandLineArgs.Add("--off-screen-frame-rate", "60");
            settings.SetOffScreenRenderingBestPerformanceArgs();

            Cef.Initialize(settings);

            CefView = new ChromiumWebBrowser("http://localhost:1337");
            CefView.DragHandler = new DragDropHandler();
            CefView.IsBrowserInitializedChanged += CefView_IsBrowserInitializedChanged;

            Controls.Add(CefView);
            CefView.Dock = DockStyle.Fill;

            CefView.JavascriptMessageReceived += OnBrowserJavascriptMessageReceived;
            //CefView.FrameLoadEnd += OnFrameLoadEnd;
        }

        public void OnFrameLoadEnd(object sender, FrameLoadEndEventArgs e)
        {
            if (e.Frame.IsMain)
            {
                //In the main frame we inject some javascript that's run on mouseUp
                //You can hook any javascript event you like.
             //   CefView.ExecuteScriptAsync(@"
	            //  document.body.onmouseup = function()
	            //  {
		           // //CefSharp.PostMessage can be used to communicate between the browser
		           // //and .Net, in this case we pass a simple string,
		           // //complex objects are supported, passing a reference to Javascript methods
		           // //is also supported.
		           // //See https://github.com/cefsharp/CefSharp/issues/2775#issuecomment-498454221 for details
		           // CefSharp.PostMessage(window.getSelection().toString());
	            //  }
	            //");
            }
        }
        private void OnBrowserJavascriptMessageReceived(object sender, JavascriptMessageReceivedEventArgs e)
        {
            CefView.Invoke((MethodInvoker)delegate
            {
                var actionValue = (IDictionary<string, object>)e.Message;
                var action = actionValue["action"].ToString();
                var eventForwarder = new EventForwarder(hwnd);
                switch (action)
                {
                    case "WindowAction":
                        Console.WriteLine("WindowAction: " + actionValue["value"]);
                        eventForwarder.WindowAction((int)actionValue["value"]);
                        break;
                    case "HideWindow":
                        Console.WriteLine("HideWindow");
                        eventForwarder.HideWindow();
                        break;
                    case "MouseResizeDrag":
                        Console.WriteLine("MouseResizeDrag: " + actionValue["value"]);
                        eventForwarder.MouseResizeDrag((int)actionValue["value"]);
                        break;
                    case "MouseDownDrag":
                        Console.WriteLine("MouseDownDrag");
                        eventForwarder.MouseDownDrag();
                        break;
                }
            });
        }
        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        public class EventForwarder
        {
            public const int WmNclButtonDown = 0xA1;
            public const int WmSysCommand = 0x112;
            public const int HtCaption = 0x2;

            [DllImport("user32.dll")]
            public static extern int SendMessage(IntPtr hWnd, int msg, int wParam, int lParam);
            [DllImport("user32.dll")]
            public static extern bool ReleaseCapture();

            readonly IntPtr _target;

            public EventForwarder(IntPtr target)
            {
                _target = target;
            }

            public void MouseDownDrag()
            {
                ReleaseCapture();
                SendMessage(_target, WmNclButtonDown, HtCaption, 0);
            }

            public void MouseResizeDrag(int wParam)
            {
                if (wParam == 0) return;
                ReleaseCapture();
                SendMessage(_target, WmNclButtonDown, wParam, 0);
            }

            public void WindowAction(int action)
            {
                ReleaseCapture();
                if (action != 0x0010) SendMessage(_target, WmSysCommand, action, 0);
                else SendMessage(_target, 0x0010, 0, 0);
            }

            public void HideWindow()
            {
                ReleaseCapture();
                SendMessage(_target, WmSysCommand, 0xF020, 0); // Minimize
                Globals.HideWindow(_target); // Hide from start bar
                Globals.StartTrayIfNotRunning();
            }
        }
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

        /// <summary>
        /// Rungs on URI change in the WebView.
        /// </summary>
        private void UrlChanged(object sender, DependencyPropertyChangedEventArgs args)
        {
            Globals.DebugWriteLine(@"[Func:(Client-CEF)MainWindow.xaml.cs.InitializeChromium]");
            Globals.WriteToLog(args.NewValue.ToString());
            var newAddress = args.NewValue.ToString();
            if (newAddress == null) return;

            if (newAddress.Contains("RESTART_AS_ADMIN")) RestartAsAdmin(newAddress.Contains("arg=") ? newAddress.Split(new[] { "arg=" }, StringSplitOptions.None)[1] : "");
            if (newAddress.Contains("EXIT_APP")) Environment.Exit(0);

            if (!newAddress.Contains("?")) return;
            // Needs to be here as:
            // Importing Microsoft.Win32 and System.Windows didn't get OpenFileDialog to work.
            var uriArg = newAddress.Split('?').Last();
            if (uriArg.StartsWith("selectFile"))
            {
                // Select file and run Model_SetFilepath()
                var argValue = uriArg.Split('=')[1];
                var dlg = new OpenFileDialog
                {
                    FileName = Path.GetFileNameWithoutExtension(argValue),
                    DefaultExt = Path.GetExtension(argValue),
                    Filter = $"{argValue}|{argValue}"
                };

                var result = dlg.ShowDialog();
                if (result != DialogResult.OK) return; // TODO: Verify that this is correct (NET to Framework)

                CefView.Load(args.OldValue.ToString()); // TODO: Verify that this cancels navigation
            }
            else if (uriArg.StartsWith("selectImage"))
            {
                // Select file and replace requested file with it.
                var imageDest = Globals.PathJoin(Globals.UserDataFolder, "wwwroot\\" + HttpUtility.UrlDecode(uriArg.Split('=')[1]));

                var dlg = new OpenFileDialog
                {
                    Filter = "Any image file (.png, .jpg, .bmp...)|*.*"
                };

                var result = dlg.ShowDialog();
                if (result != DialogResult.OK) return; // TODO: Verify that this is correct (NET to Framework)
                CefView.Load(args.OldValue.ToString()); // Cancel navigation

                File.Copy(dlg.FileName, imageDest, true);
            }
        }
        public static void RestartAsAdmin(string args)
        {
            var proc = new ProcessStartInfo
            {
                WorkingDirectory = Environment.CurrentDirectory,
                FileName = Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe") ?? "TcNo-Acc-Switcher.exe",
                UseShellExecute = true,
                Arguments = args,
                Verb = "runas"
            };
            try
            {
                _ = Process.Start(proc);
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                Environment.Exit(0);
            }
        }

        private void CefView_IsBrowserInitializedChanged(object sender, EventArgs args)
        {
            Globals.DebugWriteLine(@"[Func:(Client-CEF)MainWindow.xaml.cs.CefView_IsBrowserInitializedChanged]");
            if (!CefView.IsBrowserInitialized) return;
            CefView.ShowDevTools();

            ChromeWidgetMessageInterceptor.SetupLoop(CefView, (message) =>
            {
                if (message.Msg != WM_LBUTTONDOWN) return;
                var point = new System.Drawing.Point(message.LParam.ToInt32());

                if (!((DragDropHandler)CefView.DragHandler).DraggableRegion.IsVisible(point)) return;
                ReleaseCapture();
                SendHandleMessage();
            });
        }

        public const int WM_NCLBUTTONDOWN = 0xA1;
        public const int HT_CAPTION = 0x2;

        [System.Runtime.InteropServices.DllImport("user32.dll")]
        public static extern int SendMessage(IntPtr hWnd, int Msg, int wParam, int lParam);
        [System.Runtime.InteropServices.DllImport("user32.dll")]
        public static extern bool ReleaseCapture();

        public void SendHandleMessage()
        {
            if (InvokeRequired)
                Invoke(new SendHandleMessageDelegate(SendHandleMessage), new object[] { });
            else
                SendMessage(Handle, WM_NCLBUTTONDOWN, HT_CAPTION, 0);
        }
        public delegate void SendHandleMessageDelegate();
    }
}
