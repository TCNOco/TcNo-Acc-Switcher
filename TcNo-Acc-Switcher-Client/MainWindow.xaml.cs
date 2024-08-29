
// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Reflection;
using System.Runtime.InteropServices;
using System.Security.Principal;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Windows.Interop;
using CefSharp;
using CefSharp.Wpf;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.Wpf;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server;
using TcNo_Acc_Switcher_Server.Data;
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
        private static string _address = "";
        private readonly string _mainBrowser = AppSettings.ActiveBrowser; // <CEF/WebView>

        private static void RunServer()
        {
            const string serverPath = "TcNo-Acc-Switcher-Server_main.exe";
            if (Process.GetProcessesByName(Path.GetFileNameWithoutExtension(serverPath)).Length > 0)
            {
                Globals.WriteToLog("Server was already running. Killing process.");
                Globals.KillProcess(serverPath); // Kill server if already running
            }

            var attempts = 0;
            while (!Program.MainProgram(new[] { _address, "nobrowser" }) && attempts < 10)
            {
                Program.NewPort();
                _address = "--urls=http://localhost:" + AppSettings.ServerPort + "/";
                attempts++;
            }
            if (attempts == 10)
                MessageBox.Show("The TcNo-Acc-Switcher-Server.exe attempted to launch 10 times and failed every time. Every attempted port is taken, or another issue occurred. Check the log file for more info.", "Server start failed!", MessageBoxButton.OK, MessageBoxImage.Error);
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
                Globals.RecursiveDelete(Globals.OriginalWwwroot, false);
                Directory.Move(Path.Join(Globals.AppDataFolder, "wwwroot"), Globals.OriginalWwwroot);
            }

            Program.FindOpenPort();
            _address = "--urls=http://localhost:" + AppSettings.ServerPort + "/";

            AppData.TcNoClientApp = true;

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
                _mView2.Visibility = Visibility.Hidden;
                _mView2.Visibility = Visibility.Visible;
            }
            else
            {
                _cefView.BrowserSettings.WindowlessFrameRate = 60;
                _cefView.Visibility = Visibility.Visible;
                _cefView.Load("http://localhost:" + AppSettings.ServerPort + "/");
            }

            MainBackground.Background = App.GetStylesheetColor("headerbarBackground", "#253340");

            Width = AppSettings.WindowSize.X;
            Height = AppSettings.WindowSize.Y;
            AllowsTransparency = AppSettings.AllowTransparency;
            StateChanged += WindowStateChange;
            // Each window in the program would have its own size. IE Resize for Steam, and more.

            // Center:
            if (!AppSettings.StartCentered) return;
            Left = (SystemParameters.PrimaryScreenWidth / 2) - (Width / 2);
            Top = (SystemParameters.PrimaryScreenHeight / 2) - (Height / 2);
        }

        private ChromiumWebBrowser _cefView;
        private WebView2 _mView2;
        private void BrowserInit()
        {
            switch (_mainBrowser)
            {
                case "WebView":
                    _mView2 = new WebView2();
                    _mView2.Initialized += MView2_OnInitialised;
                    _mView2.NavigationCompleted += MView2_OnNavigationCompleted;
                    break;
                case "CEF":
                    CheckCefFiles();

                    InitializeChromium();
                    _cefView = new ChromiumWebBrowser();
                    _cefView.JavascriptMessageReceived += CefView_OnJavascriptMessageReceived;
                    _cefView.AddressChanged += CefViewOnAddressChanged;
                    _cefView.PreviewMouseUp += MainBackgroundOnPreviewMouseUp;
                    _cefView.ConsoleMessage += CefViewOnConsoleMessage;
                    _cefView.KeyboardHandler = new CefKeyboardHandler();
                    _cefView.MenuHandler = new CefMenuHandler();
                    break;
            }
        }

        private void CefViewOnConsoleMessage(object? sender, ConsoleMessageEventArgs e)
        {
            if (e.Level == LogSeverity.Error)
            {
                Globals.WriteToLog(@$"{DateTime.Now:dd-MM-yy_hh:mm:ss.fff} - CEF EXCEPTION (Handled: refreshed): {e.Message + Environment.NewLine}LINE: {e.Line + Environment.NewLine}SOURCE: {e.Source}");
                _refreshFixAttempts++;
                if (_refreshFixAttempts < 5)
                    _cefView.Reload();
                else
                    throw new Exception(
                        $"Refreshed too many times in attempt to fix issue. Error: {e.Message}");
            }
            else
            {
#if RELEASE
                try
                {
                    // ReSharper disable once PossibleNullReferenceException
                    Globals.WriteToLog(@$"{DateTime.Now:dd-MM-yy_hh:mm:ss.fff} - CEF: " + e.Message.Replace("\n", "\n\t"));
                }
                catch (Exception ex)
                {
                    Globals.WriteToLog(ex);
                }
#endif
            }
        }

        private void MainBackgroundOnPreviewMouseUp(object sender, MouseButtonEventArgs e)
        {
            if (e.ChangedButton != MouseButton.XButton1 && e.ChangedButton != MouseButton.XButton2) return;
            // Back
            e.Handled = true;
            _cefView.ExecuteScriptAsync("btnBack_Click()");
        }

        private void AddBrowser()
        {
            if (_mainBrowser == "WebView") MainBackground.Children.Add(_mView2);
            else if (_mainBrowser == "CEF") MainBackground.Children.Add(_cefView);
        }

        #region CEF
        private static void InitializeChromium()
        {
            Globals.DebugWriteLine(@"[Func:(Client-CEF)MainWindow.xaml.cs.InitializeChromium]");
            try
            {
                var settings = new CefSettings
                {
                    CachePath = Path.Join(Globals.UserDataFolder, "CEF\\Cache"),
                    UserAgent = "TcNo-CEF 1.0",
                    WindowlessRenderingEnabled = true
                };
                settings.CefCommandLineArgs.Add("-off-screen-rendering-enabled", "0");
                settings.CefCommandLineArgs.Add("--off-screen-frame-rate", "60");
                settings.SetOffScreenRenderingBestPerformanceArgs();

                Cef.Initialize(settings);
                //CefView.DragHandler = new DragDropHandler();
                //CefView.IsBrowserInitializedChanged += CefView_IsBrowserInitializedChanged;
                //CefView.FrameLoadEnd += OnFrameLoadEnd;
            } catch (Exception ex) {
                Globals.WriteToLog(ex);
                // Give warning, and open Updater with downloadCef.
                var result = MessageBox.Show("CEF (Chrome Embedded Framework) failed to load. Do you want to use WebView2 instead? (Less compatibility, more performance)\nChoosing No will verify & update the TcNo Account Switcher.\n\nIf this issue persists, visit https://tcno.co/cef.\r\n", "Missing/Outdated/Broken files!", MessageBoxButton.YesNo, MessageBoxImage.Error);
                if (result == MessageBoxResult.Yes)
                {
                    AppSettings.ActiveBrowser = "WebView";
                    AppSettings.SaveSettings();
                    Restart();
                }
                else
                {
                    if (AppSettings.OfflineMode)
                        _ = MessageBox.Show("You are in offline mode, this feature cannot be used.", "Offline Mode", MessageBoxButton.OK, MessageBoxImage.Error);
                    else
                        AppSettings.AutoStartUpdaterAsAdmin("verify");
                    Environment.Exit(1);
                }
            }
        }

        /// <summary>
        /// Check if all CEF Files are available. If not > Close and download, or revert to WebView.
        /// </summary>
        private static void CheckCefFiles()
        {
            string[] cefFiles = { "libcef.dll", "icudtl.dat", "resources.pak", "libGLESv2.dll", "d3dcompiler_47.dll", "vk_swiftshader.dll", "chrome_elf.dll", "CefSharp.BrowserSubprocess.Core.dll" };
            foreach (var cefFile in cefFiles)
            {
                var path = Path.Join(Globals.AppDataFolder, "runtimes\\win-x64\\native\\", cefFile);
                if (File.Exists(path) && new FileInfo(path).Length > 10) continue;

                var result = MessageBox.Show("CEF files not found. Download? (No reverts to WebView2, which requires WebView2 Runtime to be installed)", "Required runtime not found!", MessageBoxButton.YesNo, MessageBoxImage.Error);
                if (result == MessageBoxResult.Yes)
                {
                    AppSettings.AutoStartUpdaterAsAdmin("downloadCEF");
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

        private void CefView_OnJavascriptMessageReceived(object? sender, JavascriptMessageReceivedEventArgs e)
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
            {
                var actionValue = (IDictionary<string, object>)e.Message;
                var eventForwarder = new EventForwarder(new WindowInteropHelper(this).Handle);
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

        #region BROWSER_SHARED
        private void CefViewOnAddressChanged(object sender, DependencyPropertyChangedEventArgs e)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.UrlChanged]");
            UrlChanged(e.NewValue.ToString() ?? string.Empty);
        }
        private void MViewUrlChanged(object sender, CoreWebView2NavigationStartingEventArgs args)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.UrlChanged]");
            UrlChanged(args.Uri);
        }
        /// <summary>
        /// Runs on URI change in the WebView.
        /// </summary>
        private void UrlChanged(string uri)
        {
            // // Unused:
            // // This was originally for allowing users to activate keys for specific accounts
            // // This was never fleshed out fully, and remains just coments for now.

            //Globals.WriteToLog(uri);

            //// This is used with Steam/SteamKeys.cs for future functionality!
            //if (uri.Contains("store.steampowered.com"))
            //    _ = RunCookieCheck("steampowered.com");

            //if (uri.Contains("EXIT_APP")) Environment.Exit(0);
        }

        /// <summary>
        /// Gets all cookies, with optional filter.
        /// </summary>
        /// <returns>Cookies string "Key=Val; ..."</returns>
        private async Task<string> RunCookieCheck(string filter)
        {
            // Currently only used for Steam, but the filter is implemented for future possible functionality.

            var cookies = await _mView2.CoreWebView2.CookieManager.GetCookiesAsync(null);
            var cookiesTxt = "";
            var failedCookies = new List<string>();
            foreach (var c in cookies.Where(c => c.Domain.Contains(filter)))
            {
                if (string.IsNullOrWhiteSpace(c.Value))
                    failedCookies.Add(c.Name);
                else
                    cookiesTxt += $"{c.Name}={c.Value}; ";
            }

            // Reiterate over cookies with no values (They have values, just sometimes one or two are missed for some reason.
            foreach (var failedCookie in failedCookies)
            {
                if (cookiesTxt.Contains($"{failedCookie}=")) continue;
                var attempts = 0;
                while (attempts < 5)
                {
                    attempts++;
                    cookies = await _mView2.CoreWebView2.CookieManager.GetCookiesAsync(null);
                    if (!cookies.Any(c => c.Name == failedCookie && !string.IsNullOrWhiteSpace(c.Value))) continue;
                    cookiesTxt += $"{failedCookie}={cookies.First(c => c.Name == failedCookie).Value}; ";
                    break;
                }
            }

            // "sessionid" cookie not found? (for Steam only)
            if (filter == "steampowered.com")
            {

            }
            if (!cookiesTxt.Contains("sessionid="))
            {
                var docCookies = await _mView2.CoreWebView2.ExecuteScriptAsync("document.cookie");
                var sid = docCookies.Split("sessionid=")[1].Split(";")[0];
                if (sid[^1] == '"') sid = sid[..^1]; // If last char is quotation mark: remove
                cookiesTxt += $"sessionid={sid};";
            }
            Console.WriteLine(cookiesTxt);
            return cookiesTxt;
        }
        #endregion

        #region WebView
        private async void MView2_OnInitialised(object sender, EventArgs e)
        {
            try
            {
                var env = await CoreWebView2Environment.CreateAsync(null, Globals.UserDataFolder);
                await _mView2.EnsureCoreWebView2Async(env);
                _mView2.CoreWebView2.Settings.UserAgent = "TcNo 1.0";

                _mView2.Source = new Uri($"http://localhost:{AppSettings.ServerPort}/{App.StartPage}");
                MViewAddForwarders();
                _mView2.NavigationStarting += MViewUrlChanged;
                _mView2.CoreWebView2.ProcessFailed += CoreWebView2OnProcessFailed;

                _mView2.CoreWebView2.GetDevToolsProtocolEventReceiver("Runtime.consoleAPICalled")
                    .DevToolsProtocolEventReceived += ConsoleMessage;
                _mView2.CoreWebView2.GetDevToolsProtocolEventReceiver("Runtime.exceptionThrown")
                    .DevToolsProtocolEventReceived += ConsoleMessage;
                _ = await _mView2.CoreWebView2.CallDevToolsProtocolMethodAsync("Runtime.enable", "{}");
            }
            catch (Exception ex) when (ex is BadImageFormatException or WebView2RuntimeNotFoundException or COMException)
            {
                if (ex is COMException && !ex.ToString().Contains("WebView2"))
                {
                    // Is not a WebView2 exception
                    throw;
                }

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
                        _ = MessageBox.Show(
                            "WebView2 Runtime is not installed. The program will now download and use the fallback CEF browser. (Less performance, more compatibility)",
                            "Required runtime not found! Using fallback.", MessageBoxButton.OK, MessageBoxImage.Error);
                        AppSettings.AutoStartUpdaterAsAdmin("downloadCEF");
                        Globals.DeleteFile(failFile);
                        Environment.Exit(1);
                    }
                }

                var result =
                    MessageBox.Show(
                        "WebView2 Runtime is not installed. I've opened the website you need to download it from.",
                        "Required runtime not found!", MessageBoxButton.OKCancel, MessageBoxImage.Error);
                if (result == MessageBoxResult.OK)
                {
                    _ = Process.Start(new ProcessStartInfo("https://go.microsoft.com/fwlink/p/?LinkId=2124703")
                    {
                        UseShellExecute = true,
                        Verb = "open"
                    });
                }
                else Environment.Exit(1);
            }

            //MView2.CoreWebView2.OpenDevToolsWindow();
        }

        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        private void MViewAddForwarders()
        {
            if (_mainBrowser != "WebView") return;
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.MViewAddForwarders]");
            var eventForwarder = new EventForwarder(new WindowInteropHelper(this).Handle);

            try
            {
                _mView2.CoreWebView2.AddHostObjectToScript("eventForwarder", eventForwarder);
            }
            catch (NullReferenceException)
            {
                // To mitigate: Object reference not set to an instance of an object - Was getting a few of these a day with CrashLog reports
                if (_mView2.IsInitialized)
                    _mView2.Reload();
                else throw;
            }
            _ = _mView2.Focus();
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
            _mView2.Visibility = Visibility.Hidden;
            _mView2.Visibility = Visibility.Visible;
            _firstLoad = false;
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
                var expandedError = "";
                var details = message["exceptionDetails"];
                if (details != null)
                {
                    var ex = details.Value<JObject>("exception");
                    if (ex != null)
                    {
                        expandedError = $"{Environment.NewLine}{(string)details["url"]}:{(string)details["lineNumber"]}:{(string)details["columnNumber"]} - {(string)ex["description"]}{Environment.NewLine}{Environment.NewLine}Stack Trace:{Environment.NewLine}";
                    }

                    var stackTrace = details.Value<JObject>("stackTrace");
                    var callFrames = stackTrace?["callFrames"]?.ToObject<JArray>();
                    if (callFrames != null)
                    {
                        foreach (var callFrame in callFrames)
                        {
                            expandedError += $"    at {(string)callFrame["functionName"]} in {(string)details["url"]}:line {(string)callFrame["lineNumber"]}:{(string)callFrame["columnNumber"]}{Environment.NewLine}";
                        }
                    }
                }




                Globals.WriteToLog(@$"{DateTime.Now:dd-MM-yy_hh:mm:ss.fff} - WebView2 EXCEPTION (Handled: refreshed): {message.SelectToken("exceptionDetails.exception.description")}{Environment.NewLine}{expandedError}{Environment.NewLine}{Environment.NewLine}{Environment.NewLine}FULL ERROR: {e.ParameterObjectAsJson}");
                // Load json from string e.ParameterObjectAsJson
                _refreshFixAttempts++;
                if (_refreshFixAttempts < 5)
                    _mView2.Reload();
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
                catch (Exception ex)
                {
                    Globals.WriteToLog(ex);
                }
#endif
            }
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
            if (_mainBrowser == "WebView") _ = _mView2.ExecuteScriptAsync("document.body.classList." + state + "('maximised')");
            else if (_mainBrowser == "CEF") _cefView.ExecuteScriptAsync("document.body.classList." + state + "('maximised')");
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
                FileName = Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe") ?? "TcNo-Acc-Switcher_main.exe",
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
