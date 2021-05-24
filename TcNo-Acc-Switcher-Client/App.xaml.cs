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
using System.Globalization;
using System.IO;
using System.Net;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Threading;
using System.Windows;
using System.Windows.Input;
using System.Windows.Shapes;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using MessageBox = System.Windows.MessageBox;
using MessageBoxOptions = System.Windows.MessageBoxOptions;
using MouseEventArgs = System.Windows.Input.MouseEventArgs;
using Path = System.IO.Path;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App
    {
        public static string StartPage = "";
        
        internal static class NativeMethods
        {
            // http://msdn.microsoft.com/en-us/library/ms681944(VS.85).aspx
            // See: http://www.codeproject.com/tips/68979/Attaching-a-Console-to-a-WinForms-application.aspx
            // And: https://stackoverflow.com/questions/2669463/console-writeline-does-not-show-up-in-output-window/2669596#2669596
            [DllImport("kernel32.dll", SetLastError = true)]
            internal static extern int AllocConsole();
            [DllImport("kernel32.dll", SetLastError = true)]
            internal static extern int FreeConsole();
            [DllImport("kernel32.dll")]
            static extern IntPtr GetConsoleWindow();
            [DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
            static extern bool SetWindowText(IntPtr hwnd, String lpString);

            public static void SetWindowText(string text)
            {
                var handle = GetConsoleWindow();

                SetWindowText(handle, text);
            }
        }

        protected override void OnExit(ExitEventArgs e)
        {
            NativeMethods.FreeConsole();
            Mutex.ReleaseMutex();
        }

        public bool DebugMode = true;

        static readonly Mutex Mutex = new(true, "{A240C23D-6F45-4E92-9979-11E6CE10A22C}");
        [STAThread]
        protected override void OnStartup(StartupEventArgs e)
        {
            Directory.SetCurrentDirectory(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty); // Set working directory to same as .exe
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;

#if DEBUG
            if (DebugMode)
            {
                NativeMethods.AllocConsole();
                NativeMethods.SetWindowText("Debug console");
                Console.WriteLine("Debug Console started");
            }
#else
            // Logging:
            // Redirects all Console.WriteLines to Log.txt, which cleans itself every launch.
            var fs = new FileStream("log.txt", FileMode.Create);
            var sw = new StreamWriter(fs) {AutoFlush = true};
            Console.SetOut(sw);
            Console.SetError(sw);
#endif



            base.OnStartup(e);
            var quitArg = false;
            for (var i = 0; i != e.Args.Length; ++i)
            {
                if (e.Args[i]?[0] == '+')
                {
                    var command = e.Args[i].Substring(1).Split(':'); // Drop '+' and split
                    var platform = command[0];
                    var account = command[1];
                    
                    switch (platform)
                    {
                        case "b": // Battle.net
                            // Blizzard format: +b:<email>
                            Console.WriteLine("Battle.net switch requested");
                            TcNo_Acc_Switcher_Server.Data.Settings.BattleNet.Instance.LoadFromFile();
                            TcNo_Acc_Switcher_Server.Pages.BattleNet.BattleNetSwitcherFuncs.SwapBattleNetAccounts(account);
                            break;
                        case "o": // Origin
                            // Origin format: +o:<accName>[:<State (10 = Offline/0 = Default)>]
                            Console.WriteLine("Origin switch requested");
                            TcNo_Acc_Switcher_Server.Data.Settings.Origin.Instance.LoadFromFile();
                            TcNo_Acc_Switcher_Server.Pages.Origin.OriginSwitcherFuncs.SwapOriginAccounts(account, (command.Length > 2 ? int.Parse(command[3]) : 0));
                            break;
                        case "s": // Steam
                            // Steam format: +s:<steamId>[:<PersonaState (0-7)>]
                            Console.WriteLine("Steam switch requested");
                            TcNo_Acc_Switcher_Server.Data.Settings.Steam.Instance.LoadFromFile();
                            TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts(account.Split(":")[0], ePersonaState: (command.Length > 2 ? int.Parse(command[3]) : -1)); // Request has a PersonaState in it
                            break;
                        case "u": // Ubisoft Connect
                            // Ubisoft Connect format: +u:<email>[:<0 = Online/1 = Offline>]
                            Console.WriteLine("Ubisoft Connect switch requested");
                            TcNo_Acc_Switcher_Server.Data.Settings.Ubisoft.Instance.LoadFromFile();
                            TcNo_Acc_Switcher_Server.Pages.Ubisoft.UbisoftSwitcherFuncs.SwapUbisoftAccounts(account, (command.Length > 2 ? int.Parse(command[3]) : -1));
                            break;
                    }

                    quitArg = true;
                }
                else switch (e.Args[i])
                {
                    case "battlenet":
                        StartPage = "BattleNet"; 
                        break;
                    case "epic":
                        StartPage = "Epic";
                        break;
                    case "steam":
                        StartPage = "Steam";
                        break;
                        case "origin":
                        StartPage = "Origin";
                        break;
                    case "ubisoft":
                        StartPage = "Ubisoft";
                        break;
                    case "logout":
                        TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts();
                        quitArg = true;
                        break;
                    case "quit":
                        quitArg = true;
                        break;
                    case "v":
                    case "vv":
                    case "verbose":
                        Globals.VerboseMode = true;
                        break;
                    default:
                        Console.WriteLine($"Unknown argument: \"{e.Args[i]}\"");
                        break;
                }
            }

            if (quitArg)
                Environment.Exit(1);

            // Single instance:
            if (!Mutex.WaitOne(TimeSpan.Zero, true))
            {
                Thread.Sleep(2000); // 2 seconds before just making sure -- Might be an admin restart
                try
                {
                    if (!Mutex.WaitOne(TimeSpan.Zero, true))
                    {
                        // Try to show from tray, as user may not know it's hidden there.
                        MessageBox.Show(!Globals.BringToFront()
                            ? "Another TcNo Account Switcher instance has been detected."
                            : "TcNo Account Switcher was running.\nI've brought it to the top.\nMake sure to check your Windows Task Tray for the icon :)\n- You can exit it from there too", "TcNo Account Switcher Notice", MessageBoxButton.OK, MessageBoxImage.Information, MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);
                        Environment.Exit(1056); // 1056	An instance of the service is already running.
                    }
                }
                catch (AbandonedMutexException e2)
                {
                    // Just restarted 
                }
            }

            // See if updater was updated, and move files:
            if (Directory.Exists("newUpdater"))
            {
                try
                {
                    GeneralFuncs.RecursiveDelete(new DirectoryInfo("updater"), false);
                }
                catch (IOException ioE)
                {
                    // Catch first IOException and try to kill the updater, if it's running... Then continue.
                    Globals.KillProcess("TcNo-Acc-Switcher-Updater");
                    GeneralFuncs.RecursiveDelete(new DirectoryInfo("updater"), false);
                }
                Directory.Move("newUpdater", "updater");
            }

            // Check for update in another thread
            new Thread(CheckForUpdate).Start();
        }

        /// <summary>
        /// Checks for an update
        /// </summary>
        /// <returns>True if update found, show notification</returns>
        public static void CheckForUpdate()
        {
            try
            {
            #if DEBUG
                var latestVersion = new WebClient().DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?debug&v=" + AppSettings.Instance.Version));
            #else
                var latestVersion = new WebClient().DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?v=" + AppSettings.Instance.Version));
            #endif
                if (CheckLatest(latestVersion)) return;
                // Show notification
                AppSettings.Instance.UpdateAvailable = true;
            }
            catch (WebException e)
            {
                MessageBox.Show("Could not reach https://tcno.co/ to check for updates.", "Unable to check for updates", MessageBoxButton.OK, MessageBoxImage.Error);
                Console.WriteLine(@"Could not reach https://tcno.co/ to check for updates.\n" + e);
            }
        }

        /// <summary>
        /// Checks whether the program version is equal to or newer than the servers
        /// </summary>
        /// <param name="latest">Latest version provided by server</param>
        /// <returns>True when the program is up-to-date or ahead</returns>
        private static bool CheckLatest(string latest)
        {
            latest = latest.Replace("\r", "").Replace("\n", "");
            if (DateTime.TryParseExact(latest, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var latestDate))
            {
                if (DateTime.TryParseExact(AppSettings.Instance.Version, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var currentDate))
                {
                    if (latestDate.Equals(currentDate) || currentDate.Subtract(latestDate) > TimeSpan.Zero) return true;
                }
                else
                    Console.WriteLine(@"Unable to convert '{0}' to a date and time.", latest);
            }
            else
                Console.WriteLine(@"Unable to convert '{0}' to a date and time.", latest);
            return false;
        }


        #region ResizeWindows
        // https://stackoverflow.com/a/27157947/5165437
        private bool _resizeInProcess;
        private void Resize_Init(object sender, MouseButtonEventArgs e)
        {
            if (sender is not Rectangle senderRect) return;
            _resizeInProcess = true;
            senderRect.CaptureMouse();
        }

        private void Resize_End(object sender, MouseButtonEventArgs e)
        {
            if (sender is not Rectangle senderRect) return;
            _resizeInProcess = false;
            senderRect.ReleaseMouseCapture();
        }

        private void Resizing_Form(object sender, MouseEventArgs e)
        {
            if (!_resizeInProcess) return;
            if (!(sender is Rectangle senderRect)) return;
            var mainWindow = senderRect.Tag as Window;
            var width = e.GetPosition(mainWindow).X;
            var height = e.GetPosition(mainWindow).Y;
            senderRect.CaptureMouse();
            if (senderRect.Name.ToLower().Contains("right"))
            {
                width += 5;
                if (width > 0)
                    if (mainWindow != null)
                        mainWindow.Width = width;
            }
            if (senderRect.Name.ToLower().Contains("left"))
            {
                width -= 5;
                if (mainWindow != null)
                {
                    mainWindow.Left += width;
                    width = mainWindow.Width - width;
                    if (width > 0)
                    {
                        mainWindow.Width = width;
                    }
                }
            }
            if (senderRect.Name.ToLower().Contains("bottom"))
            {
                height += 5;
                if (height > 0)
                    if (mainWindow != null)
                        mainWindow.Height = height;
            }

            if (!senderRect.Name.ToLower().Contains("top")) return;
            height -= 5;
            if (mainWindow == null) return;
            mainWindow.Top += height;
            height = mainWindow.Height - height;
            if (height > 0)
            {
                mainWindow.Height = height;
            }
        }
        #endregion
    }
}
