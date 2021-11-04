using CefSharp;
using CefSharp.WinForms;
using System;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Web;
using System.Windows;
using System.Windows.Forms;
using TcNo_Acc_Switcher_Globals_Framework;

namespace TcNo_Acc_Switcher_Client_CEF
{
    public partial class Main : Form
    {
        public Main()
        {
            InitializeComponent();
        }

        public ChromiumWebBrowser CefView;

        // Messages
        const int WM_MOUSEMOVE = 0x0200;
        const int WM_MOUSELEAVE = 0x02A3;
        const int WM_LBUTTONDOWN = 0x0201;
        const int WM_LBUTTONUP = 0x0202;

        private void Main_Load(object sender, EventArgs e)
        {
            InitializeChromium();
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
