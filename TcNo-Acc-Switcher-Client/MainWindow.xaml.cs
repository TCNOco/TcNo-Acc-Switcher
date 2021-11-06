﻿
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
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net.NetworkInformation;
using System.Reflection;
using System.Security.Principal;
using System.Threading;
using System.Web;
using System.Windows;
using System.Windows.Forms;
using System.Windows.Input;
using System.Windows.Interop;
using System.Windows.Media;
using CefSharp;
using CefSharp.Wpf;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.Wpf;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared;
using MessageBox = System.Windows.MessageBox;
using MessageBoxOptions = System.Windows.MessageBoxOptions;
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
        private string _mainBrowser = AppSettings.ActiveBrowser; // <CEF/WebView>

        private static void RunServer()
        {
            const string serverPath = "TcNo-Acc-Switcher-Server.exe";
            if (Process.GetProcessesByName(Path.GetFileNameWithoutExtension(serverPath)).Length > 0)
            {
                Globals.WriteToLog("Server was already running. Killing process.");
                Globals.KillProcess(serverPath); // Kill server if already running
            }

            var attempts = 0;
            Exception last;
            while (!Program.MainProgram(new[] { _address }, out last) && attempts < 10)
            {
                NewPort();
                _address = "--urls=http://localhost:" + AppSettings.ServerPort + "/";
                attempts++;
            }
            if (attempts == 10 && last != null)
                throw last;
        }

        private static bool IsAdmin()
        {
            // Checks whether program is running as Admin or not
            var securityIdentifier = WindowsIdentity.GetCurrent().Owner;
            return securityIdentifier is not null && securityIdentifier.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid);
        }

        public MainWindow()
        {
            // Set working directory to documents folder
            Directory.SetCurrentDirectory(Globals.UserDataFolder);

            if (Directory.Exists(Path.Join(Globals.AppDataFolder, "wwwroot")))
            {
                if (Globals.InstalledToProgramFiles() && !IsAdmin() || !Globals.HasFolderAccess(Globals.AppDataFolder))
                    Restart("", true);
                if (Directory.Exists(Globals.OriginalWwwroot)) GeneralFuncs.RecursiveDelete(new DirectoryInfo(Globals.OriginalWwwroot), false);
                Directory.Move(Path.Join(Globals.AppDataFolder, "wwwroot"), Globals.OriginalWwwroot);
            }

            FindOpenPort();
            _address = "--urls=http://localhost:" + AppSettings.ServerPort + "/";

            // Start web server
            Server.IsBackground = true;
            Server.Start();

            // Initialize correct browser
            BrowserInit();

            // Initialise and connect to web server above
            InitializeComponent();
            AddBrowser();

            if (_mainBrowser == "WebView")
            {
                // Attempt to fix window showing as blank.
                // See https://github.com/MicrosoftEdge/WebView2Feedback/issues/1077#issuecomment-856222593593
                MView2.Visibility = Visibility.Hidden;
                MView2.Visibility = Visibility.Visible;
            }
            else
            {
                CefView.BrowserSettings.WindowlessFrameRate = 60;
                CefView.Visibility = Visibility.Visible;
                CefView.Load("http://localhost:" + AppSettings.ServerPort + "/");
            }

            MainBackground.Background = App.GetStylesheetColor("headerbarBackground", "#253340");

            Width = AppSettings.WindowSize.X;
            Height = AppSettings.WindowSize.Y;
            StateChanged += WindowStateChange;
            // Each window in the program would have its own size. IE Resize for Steam, and more.

            // Center:
            if (AppSettings.Instance.StartCentered)
            {
                Left = (SystemParameters.PrimaryScreenWidth / 2) - (Width / 2);
                Top = (SystemParameters.PrimaryScreenHeight / 2) - (Height / 2);
            }
        }

        private ChromiumWebBrowser CefView;
        private WebView2 MView2;
        private void BrowserInit()
        {
            if (_mainBrowser == "WebView")
            {
                MView2 = new WebView2();
                MView2.Initialized += MView2_OnInitialised;
                MView2.NavigationCompleted += MView2_OnNavigationCompleted;
            }
            else if (_mainBrowser == "CEF")
            {
                CheckCefFiles();

                InitializeChromium();
                CefView = new ChromiumWebBrowser();
                CefView.JavascriptMessageReceived += CefView_OnJavascriptMessageReceived;
                CefView.AddressChanged += CefViewOnAddressChanged;
                CefView.PreviewMouseUp += MainBackgroundOnPreviewMouseUp;
                CefView.KeyboardHandler = new CefKeyboardHandler();
            }
        }

        private void MainBackgroundOnPreviewMouseUp(object sender, MouseButtonEventArgs e)
        {
            if (e.ChangedButton != MouseButton.XButton1 && e.ChangedButton != MouseButton.XButton2) return;
            // Back
            e.Handled = true;
            CefView.ExecuteScriptAsync("btnBack_Click()");
        }

        private void AddBrowser()
        {
            if (_mainBrowser == "WebView") MainBackground.Children.Add(MView2);
            else if (_mainBrowser == "CEF") MainBackground.Children.Add(CefView);
        }

        #region CEF
        private void InitializeChromium()
        {
            Globals.DebugWriteLine(@"[Func:(Client-CEF)MainWindow.xaml.cs.InitializeChromium]");
            CefSettings settings = new CefSettings()
            {
                CachePath = Path.Join(Globals.UserDataFolder, "CEF\\Cache"),
                UserAgent = "TcNo-CEF 1.0",
                UserDataPath = Path.Join(Globals.UserDataFolder, "CEF\\Data"),
                WindowlessRenderingEnabled = false
            };
            settings.CefCommandLineArgs.Add("-off-screen-rendering-enabled", "0");
            settings.CefCommandLineArgs.Add("--off-screen-frame-rate", "60");
            settings.SetOffScreenRenderingBestPerformanceArgs();

            Cef.Initialize(settings);
            //CefView.DragHandler = new DragDropHandler();
            //CefView.IsBrowserInitializedChanged += CefView_IsBrowserInitializedChanged;
            //CefView.FrameLoadEnd += OnFrameLoadEnd;
        }

        /// <summary>
        /// Check if all CEF Files are available. If not > Close and download, or revert to WebView.
        /// </summary>
        private void CheckCefFiles()
        {
            string[] CefFiles = { "libcef.dll", "icudtl.dat", "resources.pak", "libGLESv2.dll", "d3dcompiler_47.dll", "vk_swiftshader.dll", "CefSharp.dll", "chrome_elf.dll", "CefSharp.BrowserSubprocess.Core.dll" };
            foreach (var cefFile in CefFiles)
            {
                if (File.Exists(Path.Join(Globals.AppDataFolder, "runtimes\\win-x64\\native\\", cefFile))) continue;

                var result = MessageBox.Show("CEF files not found. Download? (No reverts to WebView2, which requires WebView2 Runtime to be installed)", "Required runtime not found!", MessageBoxButton.YesNo, MessageBoxImage.Error);
                if (result == MessageBoxResult.Yes)
                {
                    TcNo_Acc_Switcher_Server.Pages.Index.AutoStartUpdaterAsAdmin("downloadCEF");
                    Environment.Exit(1);
                }
                else
                {
                    AppSettings.ActiveBrowser = "WebView";
                    AppSettings.SaveSettings();
                    Restart();
                }
            }
        }

        private void CefViewOnAddressChanged(object sender, DependencyPropertyChangedEventArgs e)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.UrlChanged]");
            UrlChanged(e.NewValue.ToString());
        }

        private void CefView_OnJavascriptMessageReceived(object? sender, JavascriptMessageReceivedEventArgs e)
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
            {
                var actionValue = (IDictionary<string, object>)e.Message;
                var eventForwarder = new Headerbar.EventForwarder(new WindowInteropHelper(this).Handle);
                switch (actionValue["action"].ToString())
                {
                    case "WindowAction":
                        eventForwarder.WindowAction((int)actionValue["value"]);
                        break;
                    case "HideWindow":
                        eventForwarder.HideWindow();
                        break;
                    case "MouseResizeDrag":
                        eventForwarder.MouseResizeDrag((int)actionValue["value"]);
                        break;
                    case "MouseDownDrag":
                        eventForwarder.MouseDownDrag();
                        break;
                }
            }));
        }
        #endregion


        #region WebView
        private async void MView2_OnInitialised(object sender, EventArgs e)
        {
            try
            {
                var env = await CoreWebView2Environment.CreateAsync(null, Globals.UserDataFolder);
                await MView2.EnsureCoreWebView2Async(env);
                MView2.CoreWebView2.Settings.UserAgent = "TcNo 1.0";

                MView2.Source = new Uri($"http://localhost:{AppSettings.ServerPort}/{App.StartPage}");
                MViewAddForwarders();
                MView2.NavigationStarting += MViewUrlChanged;
                MView2.CoreWebView2.ProcessFailed += CoreWebView2OnProcessFailed;

                MView2.CoreWebView2.GetDevToolsProtocolEventReceiver("Runtime.consoleAPICalled").DevToolsProtocolEventReceived += ConsoleMessage;
                MView2.CoreWebView2.GetDevToolsProtocolEventReceiver("Runtime.exceptionThrown").DevToolsProtocolEventReceived += ConsoleMessage;
                _ = await MView2.CoreWebView2.CallDevToolsProtocolMethodAsync("Runtime.enable", "{}");
            }
            catch (WebView2RuntimeNotFoundException)
            {
                // WebView2 is not installed!
                // Create counter for WebView failed checks
                var failFile = Path.Join(Globals.UserDataFolder, "WebViewNotInstalled");
                if (!File.Exists(failFile))
                    await File.WriteAllTextAsync(failFile, "1");
                else
                {
                    if (await File.ReadAllTextAsync(failFile) == "1") await File.WriteAllTextAsync(failFile, "2");
                    else
                    {
                        AppSettings.ActiveBrowser = "CEF";
                        AppSettings.SaveSettings();
                        _ = MessageBox.Show("WebView2 Runtime is not installed. The program will now download and use the fallback CEF browser. (Less performance, more compatibility)", "Required runtime not found! Using fallback.", MessageBoxButton.OK, MessageBoxImage.Error);
                        TcNo_Acc_Switcher_Server.Pages.Index.AutoStartUpdaterAsAdmin("downloadCEF");
                        File.Delete(failFile);
                        Environment.Exit(1);
                    }
                }

                var result = MessageBox.Show("WebView2 Runtime is not installed. I've opened the website you need to download it from.", "Required runtime not found!", MessageBoxButton.OKCancel, MessageBoxImage.Error);
                if (result == MessageBoxResult.OK)
                {
                    _ = Process.Start(new ProcessStartInfo("https://go.microsoft.com/fwlink/p/?LinkId=2124703")
                    {
                        UseShellExecute = true,
                        Verb = "open"
                    });
                } else Environment.Exit(1);
            }
            //MView2.CoreWebView2.OpenDevToolsWindow();
        }

        private void MViewUrlChanged(object sender, CoreWebView2NavigationStartingEventArgs args)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.UrlChanged]");
            UrlChanged(args.Uri, args);
        }

        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        private void MViewAddForwarders()
        {
            if (_mainBrowser != "WebView") return;
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.MViewAddForwarders]");
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
            _ = MView2.Focus();
        }
        private void CoreWebView2OnProcessFailed(object? sender, CoreWebView2ProcessFailedEventArgs e)
        {
            _ = MessageBox.Show("The WebView browser process has crashed! The program will now exit.", "Fatal error", MessageBoxButton.OK,
                MessageBoxImage.Error,
                MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);
            Environment.Exit(1);
        }

        private static bool _firstLoad = true;
        private void MView2_OnNavigationCompleted(object? sender, CoreWebView2NavigationCompletedEventArgs e)
        {
            if (!_firstLoad) return;
            MView2.Visibility = Visibility.Hidden;
            MView2.Visibility = Visibility.Visible;
            _firstLoad = false;
        }
        #endregion

        #region BROWSER_SHARED

        /// <summary>
        /// Rungs on URI change in the WebView.
        /// </summary>
        private void UrlChanged(string uri, CoreWebView2NavigationStartingEventArgs mViewArgs = null)
        {
            Globals.WriteToLog(uri);

            if (uri.Contains("RESTART_AS_ADMIN")) Restart(uri.Contains("arg=") ? uri.Split("arg=")[1] : "", true);
            if (uri.Contains("EXIT_APP")) Environment.Exit(0);

            if (!uri.Contains("?")) return;
            // Needs to be here as:
            // Importing Microsoft.Win32 and System.Windows didn't get OpenFileDialog to work.
            var uriArg = uri.Split("?").Last();
            if (uriArg.StartsWith("selectFile"))
            {
                // Select file and run Model_SetFilepath()
                if (mViewArgs != null) mViewArgs.Cancel = true;
                var argValue = uriArg.Split("=")[1];
                var dlg = new OpenFileDialog
                {
                    FileName = Path.GetFileNameWithoutExtension(argValue),
                    DefaultExt = Path.GetExtension(argValue),
                    Filter = $"{argValue}|{argValue}"
                };

                var result = dlg.ShowDialog();
                if (result != true) return;
                if (_mainBrowser == "WebView")
                {
                    _ = MView2.ExecuteScriptAsync("Modal_RequestedLocated(true)");
                    _ = MView2.ExecuteScriptAsync("Modal_SetFilepath(" +
                                                  JsonConvert.SerializeObject(dlg.FileName[..dlg.FileName.LastIndexOf('\\')]) + ")");
                } else if (_mainBrowser == "CEF")
                {
                    CefView.ExecuteScriptAsync("Modal_RequestedLocated(true)");
                    CefView.ExecuteScriptAsync("Modal_SetFilepath(" +
                                               JsonConvert.SerializeObject(dlg.FileName[..dlg.FileName.LastIndexOf('\\')]) + ")");
                }
            }
            else if (uriArg.StartsWith("selectImage"))
            {
                // Select file and replace requested file with it.
                if (mViewArgs != null) mViewArgs.Cancel = true;
                var imageDest = Path.Join(Globals.UserDataFolder, "wwwroot\\" + HttpUtility.UrlDecode(uriArg.Split("=")[1]));

                var dlg = new OpenFileDialog
                {
                    Filter = "Any image file (.png, .jpg, .bmp...)|*.*"
                };

                var result = dlg.ShowDialog();
                if (result != true) return;
                File.Copy(dlg.FileName, imageDest, true);

                if (_mainBrowser == "WebView") MView2.Reload();
                else if (_mainBrowser == "CEF") CefView.Reload();
            }
        }
        #endregion

        private int _refreshFixAttempts;

        /// <summary>
        /// Handles console messages, and logs them to a file
        /// </summary>
        /// <param name="sender"></param>
        /// <param name="e"></param>
        private void ConsoleMessage(object sender, CoreWebView2DevToolsProtocolEventReceivedEventArgs e)
        {
            if (_mainBrowser != "WebView") return;
            if (e?.ParameterObjectAsJson == null) return;
            var message = JObject.Parse(e.ParameterObjectAsJson);
            if (message.ContainsKey("exceptionDetails"))
            {
                Globals.WriteToLog(@$"{DateTime.Now:dd-MM-yy_hh:mm:ss.fff} - WebView2 EXCEPTION (Handled: refreshed): " + message.SelectToken("exceptionDetails.exception.description"));
                _refreshFixAttempts++;
                if (_refreshFixAttempts < 5)
                    MView2.Reload();
                else throw new Exception($"Refreshed too many times in attempt to fix issue. Error: {message.SelectToken("exceptionDetails.exception.description")}");
            }
            else
            {
#if RELEASE
                try
                {
                    // ReSharper disable once PossibleNullReferenceException
                    foreach (var jo in message.SelectToken("args"))
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
                if (tcpConnInfoArray.All(x => x.LocalEndPoint.Port != AppSettings.ServerPort)) break;
                NewPort();
            }
        }

        private static void NewPort()
        {
            var r = new Random();
            AppSettings.ServerPort = r.Next(20000, 40000); // Random int [Why this range? See: https://www.sciencedirect.com/topics/computer-science/registered-port & netsh interface ipv4 show excludedportrange protocol=tcp]
            AppSettings.SaveSettings();
        }

        /// <summary>
        /// Rungs on WindowStateChange, to update window controls in the WebView2.
        /// </summary>
        private void WindowStateChange(object sender, EventArgs e)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.WindowStateChange]");

            var state = WindowState switch
            {
                WindowState.Maximized => "add",
                WindowState.Normal => "remove",
                _ => ""
            };
            if (_mainBrowser == "WebView") _ = MView2.ExecuteScriptAsync("document.body.classList." + state + "('maximised')");
            else if (_mainBrowser == "CEF") CefView.ExecuteScriptAsync("document.body.classList." + state + "('maximised')");
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

        public static void Restart(string args = "", bool admin = false)
        {
            var proc = new ProcessStartInfo
            {
                WorkingDirectory = Environment.CurrentDirectory,
                FileName = Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe") ?? "TcNo-Acc-Switcher.exe",
                UseShellExecute = true,
                Arguments = args,
                Verb = admin ? "runas" : ""
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
    }
}
