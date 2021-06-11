
// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net.NetworkInformation;
using System.Reflection;
using System.Threading;
using System.Web;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.Wpf;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Shared;
using OpenFileDialog = Microsoft.Win32.OpenFileDialog;
using Point = System.Drawing.Point;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>

    // This is reported as "never used" by JetBrains Inspector... what?
    public partial class MainWindow
    {
        private static readonly Thread Server = new(RunServer);
        public static readonly AppSettings AppSettings = AppSettings.Instance;
        private static string _address = "";

        private static void RunServer()
        {
            const string serverPath = "TcNo-Acc-Switcher-Server.exe";
            if (Process.GetProcessesByName(Path.GetFileNameWithoutExtension(serverPath)).Length > 0)
            {
                Globals.WriteToLog("Server was already running. Killing process."); 
                Globals.KillProcess(serverPath); // Kill server if already running
            }

            var attempts = 0;
            var last = new IOException();
            while (attempts < 10)
            {
                try
                {
                    Program.Main(new[] { _address });
                    return;
                }
                catch (IOException e)
                {
                    last = e;
                    NewPort();
                    _address = "--urls=http://localhost:" + AppSettings.ServerPort + "/";
                    attempts++;
                }
            }

            throw last;
        }
        
        public MainWindow()
        {
            Directory.SetCurrentDirectory(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty); // Set working directory to same as .exe
            
            AppSettings.LoadFromFile();
            FindOpenPort();
            _address = "--urls=http://localhost:" + AppSettings.ServerPort + "/";

            // Start web server
            Server.IsBackground = true;
            Server.Start();

            // Initialise and connect to web server above
            // Somehow check ports and find a different one if it doesn't work? We'll see...
            InitializeComponent();
            
            MainBackground.Background = (Brush)new BrushConverter().ConvertFromString(AppSettings.Stylesheet["headerbarBackground"]);
            
            Width = AppSettings.WindowSize.X;
            Height = AppSettings.WindowSize.Y;
            StateChanged += WindowStateChange;
            // Each window in the program would have its own size. IE Resize for Steam, and more.
        }

        private async void MView2_OnInitialised(object sender, EventArgs e)
        {
            MView2.CoreWebView2InitializationCompleted += WebView_CoreWebView2Ready;
            await MView2.EnsureCoreWebView2Async();
            MView2.Source = new Uri($"http://localhost:{AppSettings.ServerPort}/{App.StartPage}");
            //MView2.Source = new Uri($"http://localhost:{AppSettings.ServerPort}/{App.StartPage}");
            MView2.NavigationStarting += UrlChanged;
            //MView2.MouseDown += MViewMDown;


            MView2.CoreWebView2.GetDevToolsProtocolEventReceiver("Runtime.consoleAPICalled").DevToolsProtocolEventReceived += ConsoleMessage;
            MView2.CoreWebView2.GetDevToolsProtocolEventReceiver("Runtime.exceptionThrown").DevToolsProtocolEventReceived += ConsoleMessage;
            await MView2.CoreWebView2.CallDevToolsProtocolMethodAsync("Runtime.enable", "{}");
            //MView2.CoreWebView2.OpenDevToolsWindow();
        }

        /// <summary>
        /// Handles console messages, and logs them to a file
        /// </summary>
        /// <param name="sender"></param>
        /// <param name="e"></param>
        private void ConsoleMessage(object sender, CoreWebView2DevToolsProtocolEventReceivedEventArgs e)
        {
            if (e?.ParameterObjectAsJson == null) return;
            var message = JObject.Parse(e.ParameterObjectAsJson);
            if (message.ContainsKey("exceptionDetails"))
            {
                Globals.WriteToLog(@$"{DateTime.Now:dd-MM-yy_hh:mm:ss.fff} - WebView2 EXCEPTION (Handled: refreshed): " + message.SelectToken("exceptionDetails.exception.description"));
                MView2.Reload();
            }
            else
            {
#if RELEASE
                try
                {
                    // ReSharper disable once PossibleNullReferenceException
                    foreach (var jo in message?.SelectToken("args"))
                    {
                        Globals.WriteToLog(@$"{DateTime.Now:dd-MM-yy_hh:mm:ss.fff} - WebView2: " + jo.SelectToken("value")?.ToString().Replace("\n", "\n\t"));
                    }
                }
                catch (Exception exception)
                {
                    Globals.WriteToLog(exception.ToString());
                }
#endif
            }
        }

        /// <summary>
        /// Find first available port up from requested
        /// </summary>
        private static void FindOpenPort()
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.FindOpenPort]");
            // Check if port available:
            var ipGlobalProperties = IPGlobalProperties.GetIPGlobalProperties();
            var tcpConnInfoArray = ipGlobalProperties.GetActiveTcpConnections();
            while (true)
            {
                if (tcpConnInfoArray.Count(x => x.LocalEndPoint.Port == AppSettings.ServerPort) == 0) break;
                NewPort();
            }
        }

        private static void NewPort()
        {
            var r = new Random();
            AppSettings.ServerPort = r.Next(20000, 40000); // Random int [Why this range? See: https://www.sciencedirect.com/topics/computer-science/registered-port & netsh interface ipv4 show excludedportrange protocol=tcp]
            AppSettings.SaveSettings();
        }

        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        private void WebView_CoreWebView2Ready(object sender, EventArgs e)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.WebView_CoreWebView2Ready]");
            var eventForwarder = new Headerbar.EventForwarder(new WindowInteropHelper(this).Handle);

            try
            {
                MView2.CoreWebView2.AddHostObjectToScript("eventForwarder", eventForwarder);
            }
            catch (NullReferenceException)
            {
                // To mitigate: Object reference not set to an instance of an object - Was getting a few of these a day with CrashLog reports
                if (MView2.IsInitialized)
                    MView2.Reload();
                else throw;
            }
            MView2.Focus();
        }

        /// <summary>
        /// Rungs on WindowStateChange, to update window controls in the WebView2.
        /// </summary>
        private void WindowStateChange(object sender, EventArgs e)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.WindowStateChange]");

            if (AppSettings.TrayMinimizeLessMem) new Thread(CheckVisibility).Start();

            var state = WindowState switch
            {
                WindowState.Maximized => "add",
                WindowState.Normal => "remove",
                _ => ""
            };
            MView2.ExecuteScriptAsync("document.body.classList." + state + "('maximised')");
        }

        private void CheckVisibility()
        {
            Thread.Sleep(100);
            // While this could handle WindowState == WindowState.Minimised/Normal etc, it's only going to work off the hidden part.
            // As currently this is only going to create/dispose the WebView for better performance when minimising to tray.
            Dispatcher.Invoke(() =>
            {
                // Check if hidden or not:
                var windowLong = Globals.GetWindow(new WindowInteropHelper(this).Handle);
                if (windowLong == (windowLong & ~Globals.WsExAppWindow)) // Hidden
                {
                    MainBackground.Children.Remove(MView2);
                    MView2.Dispose();
                    GC.WaitForPendingFinalizers();
                }
                else // Not hidden
                {
                    MView2 = new WebView2();
                    MView2.Initialized += MView2_OnInitialised;
                    MainBackground.Children.Add(MView2);
                    MView2.BeginInit();
                }
            });
        }

        /// <summary>
        /// Saves window size when closing.
        /// </summary>
        protected override void OnClosing(CancelEventArgs e)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.OnClosing]");
            AppSettings.WindowSize = new Point { X = Convert.ToInt32(Width), Y = Convert.ToInt32(Height) };
            AppSettings.SaveSettings();
        }

        /// <summary>
        /// Rungs on URI change in the WebView.
        /// </summary>
        private void UrlChanged(object sender, CoreWebView2NavigationStartingEventArgs args)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.UrlChanged]");
            Globals.WriteToLog(args.Uri);

            if (args.Uri.Contains("RESTART_AS_ADMIN")) RestartAsAdmin((args.Uri.Contains("arg=") ? args.Uri.Split("arg=")[1] : ""));

            if (!args.Uri.Contains("?")) return;
            // Needs to be here as:
            // Importing Microsoft.Win32 and System.Windows didn't get OpenFileDialog to work.
            var uriArg = args.Uri.Split("?").Last();
            if (uriArg.StartsWith("selectFile"))
            {
                // Select file and run Model_SetFilepath()
                args.Cancel = true;
                var argValue = uriArg.Split("=")[1];
                var dlg = new OpenFileDialog
                {
                    FileName = Path.GetFileNameWithoutExtension(argValue),
                    DefaultExt = Path.GetExtension(argValue),
                    Filter = $"{argValue}|{argValue}"
                };

                var result = dlg.ShowDialog();
                if (result != true) return;
                MView2.ExecuteScriptAsync("Modal_RequestedLocated(true)");
                MView2.ExecuteScriptAsync("Modal_SetFilepath(" +
                                          JsonConvert.SerializeObject(dlg.FileName.Substring(0,
                                              dlg.FileName.LastIndexOf('\\'))) + ")");
            }
            else if (uriArg.StartsWith("selectImage"))
            {
                // Select file and replace requested file with it.
                args.Cancel = true;
                var imageDest = "wwwroot\\" + HttpUtility.UrlDecode(uriArg.Split("=")[1]);
                
                var dlg = new OpenFileDialog
                {
                    Filter = $"Any image file (.png, .jpg, .bmp...)|*.*"
                };

                var result = dlg.ShowDialog();
                if (result != true) return;
                File.Copy(dlg.FileName, imageDest, true);
                MView2.Reload();
            }

        }

        public static void RestartAsAdmin(string args = "")
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
                Process.Start(proc);
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                Environment.Exit(0);
            }
        }
    }
}
