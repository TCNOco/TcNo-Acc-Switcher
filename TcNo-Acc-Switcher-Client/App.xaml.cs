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
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net.Http;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Windows.Media;
using CefSharp;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using static TcNo_Acc_Switcher_Client.MainWindow;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App
    {
#pragma warning disable CA2211 // Non-constant fields should not be visible - Accessed from App.xaml.cs
        public static string StartPage = "";
#pragma warning restore CA2211 // Non-constant fields should not be visible
        private static readonly HttpClient Client = new();

        protected override void OnExit(ExitEventArgs e)
        {
            _ = NativeFuncs.FreeConsole();
#if DEBUG
            try
            {
#endif
                Mutex.ReleaseMutex();
#if DEBUG
            }
            catch
            {
                // Ignore errors if run in debug mode
            }
#endif
        }

        private static readonly Mutex Mutex = new(true, "{A240C23D-6F45-4E92-9979-11E6CE10A22C}");

        [STAThread]
        protected override async void OnStartup(StartupEventArgs e)
        {
            // Ensure files in documents are available.
            Globals.CreateDataFolder(false);

            // Remove leftover files from previous operations
            CleanupAppFolder();

            Directory.SetCurrentDirectory(Globals.UserDataFolder);

            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
            // Upload crash logs if any, before starting program
            MoveLogs();

            if (e.Args.Length != 0) // An argument was passed
            {
                // Iterate through arguments
                foreach (var eArg in e.Args)
                {
                    // Check if arguments are a platform
                    foreach (var p in AppData.Instance.PlatformList.Where(p => p.Equals(eArg, StringComparison.InvariantCultureIgnoreCase)))
                    {
                        StartPage = p;
                        break;
                    }

                    // Check if platform short is BasicPlatform
                    if (BasicPlatforms.PlatformExistsFromShort(eArg))
                        StartPage = "Basic?plat=" + eArg;

                    // Check if it was verbose mode.
                    Globals.VerboseMode = Globals.VerboseMode || eArg is "v" or "vv" or "verbose";
                }


                // Was not asked to open a platform screen, or was NOT just the verbose mode argument
                // - Therefore: It was a CLI command
                // - But, do check if the tcno:\\ cli command was issued, and only that - If it was: start the program.
                if (StartPage == "" && !(Globals.VerboseMode && e.Args.Length == 1) && !(e.Args.Length == 1 && (e.Args[0] == @"tcno:\\" || e.Args[0] == "tcno:\\")))
                {
                    if (!NativeFuncs.AttachToConsole(-1)) // Attach to a parent process console (ATTACH_PARENT_PROCESS)
                        _ = NativeFuncs.AllocConsole();
                    Console.WriteLine(Environment.NewLine);
                    var shouldClose = await ConsoleMain(e).ConfigureAwait(false);
                    if (shouldClose)
                    {
                        Console.WriteLine(Environment.NewLine + @"Press any key to close this window...");
                        _ = NativeFuncs.FreeConsole();
                        Environment.Exit(0);
                        return;
                    }

                    _ = NativeFuncs.FreeConsole();
                }

            }

#if DEBUG
            _ = NativeFuncs.AllocConsole();
            NativeFuncs.SetWindowText("Debug console");
            Globals.WriteToLog("Debug Console started");
#endif
            try
            {
                Globals.ClearLogs();
            }
            catch (UnauthorizedAccessException)
            {
                MessageBox.Show("Can't access log.txt in the TcNo Account Switcher directory!",
                    "Failed to access files", MessageBoxButton.OK, MessageBoxImage.Error);
                Environment.Exit(4); // The system cannot open the file.
            }

            // Key being held down?
            if ((Keyboard.Modifiers & ModifierKeys.Control) > 0 || (Keyboard.Modifiers & ModifierKeys.Alt) > 0 ||
                (Keyboard.Modifiers & ModifierKeys.Shift) > 0 ||
                (Keyboard.GetKeyStates(Key.Scroll) & KeyStates.Down) != 0)
            {
                // This can be improved. Somehow ignore self, and make sure all processes are killed before self.
                if (GeneralFuncs.CanKillProcess("TcNo"))
                    Globals.KillProcess("TcNo");
            }

            // Single instance:
            IsRunningAlready();

            // See if updater was updated, and move files:
            if (Directory.Exists("newUpdater"))
            {
                try
                {
                    Globals.RecursiveDelete("updater", false);
                }
                catch (IOException)
                {
                    // Catch first IOException and try to kill the updater, if it's running... Then continue.
                    Globals.KillProcess("TcNo-Acc-Switcher-Updater");
                    Globals.RecursiveDelete("updater", false);
                }

                Directory.Move("newUpdater", "updater");
            }

            //// Clear WebView2 cache
            //// This is disabled for now in hopes of fixing cache errors in console, causing it to launch with some issues.
            //Globals.ClearWebCache();

            // Show window (Because no command line commands were parsed)
            try
            {
                var mainWindow = new MainWindow();
                mainWindow.Show();
            }
            catch (FileNotFoundException ex)
            {
                // Check if CEF issue, and download if missing.
                if (!ex.ToString().Contains("CefSharp")) throw;
                AppSettings.AutoStartUpdaterAsAdmin("downloadCEF");
                Environment.Exit(1);
                throw;
            }

            if (!File.Exists("LastError.txt")) return;

            var lastError = await File.ReadAllLinesAsync("LastError.txt");
            lastError = lastError.Skip(1).ToArray();
            // TODO: Work in progress:
            //ShowErrorMessage("Error from last crash", "Last error message:" + Environment.NewLine + string.Join(Environment.NewLine, lastError));
            MessageBox.Show("Last error message:" + Environment.NewLine + string.Join(Environment.NewLine, lastError), "Error from last crash", MessageBoxButton.OK, MessageBoxImage.Error, MessageBoxResult.OK);
            Globals.DeleteFile("LastError.txt");

            AppStats.CrashCount++;
            AppStats.SaveSettings();
        }

        /// <summary>
        /// Cleanup leftover files from previous operations
        /// </summary>
        private static void CleanupAppFolder()
        {
            var backupTemp = Path.Join(Globals.UserDataFolder, "BackupTemp");
            if (Directory.Exists(backupTemp)) Globals.RecursiveDelete(backupTemp, false);
            var restoreTemp = Path.Join(Globals.UserDataFolder, "Restore");
            if (Directory.Exists(restoreTemp)) Globals.RecursiveDelete(restoreTemp, false);
        }

/*
        private static void ShowErrorMessage(string title, string text)
        {
            var cmb = new CustomMessageBox(title, text)
            {
                Topmost = true,
                Resources =
                {
                    ["HeaderbarBackground"] = GetStylesheetColor("headerbarBackground", "#14151E"),
                    ["HeaderbarBackground"] = GetStylesheetColor("windowTitleColor", "#FFFFFF"),
                    ["MainBackground"] = GetStylesheetColor("mainBackground", "#28293A"),
                    ["MessageForeground"] = GetStylesheetColor("defaultTextColor", "#FFFFFF"),
                    ["IconFill"] = GetStylesheetColor("popupIconErrorFill", "red"),
                    ["BtnBackground"] = GetStylesheetColor("buttonBackground", "#333333"),
                    ["BtnBackgroundHover"] = GetStylesheetColor("buttonBackground-hover", "#888888"),
                    ["BtnBackgroundActive"] = GetStylesheetColor("buttonBackground-active", "#888888"),
                    ["BtnForeground"] = GetStylesheetColor("buttonColor", "#FFFFFF"),
                    ["BtnBorder"] = GetStylesheetColor("buttonBorderMessageForeground", "#888888"),
                    ["BtnBorderHover"] = GetStylesheetColor("buttonBorder-hover", "#888888"),
                    ["BtnBorderActive"] = GetStylesheetColor("buttonBorder-active", "#FFAA00"),
                    ["MinBackground"] = GetStylesheetColor("windowControlsBackground", "#14151E"),
                    ["MinBackgroundHover"] = GetStylesheetColor("windowControlsBackground-hover", "#181924"),
                    ["MinBackgroundActive"] = GetStylesheetColor("windowControlsBackground-active", "#1f202e"),
                    ["CloseBackgroundHover"] = GetStylesheetColor("windowControlsCloseBackground", "#E81123"),
                    ["CloseBackgroundActive"] = GetStylesheetColor("windowControlsCloseBackground-active", "#F1707A")
                }
            };

            _ = cmb.ShowDialog();
        }
*/

        public static SolidColorBrush GetStylesheetColor(string key, string fallback)
        {
            string color;
            try
            {
                var start = AppSettings.Stylesheet.IndexOf(key + ":", StringComparison.Ordinal) + key.Length + 1;
                var end = AppSettings.Stylesheet.IndexOf(";", start, StringComparison.Ordinal);
                color = AppSettings.Stylesheet[start..end];
                color = color.Trim(); // Remove whitespace around variable
            }
            catch (Exception)
            {
                color = "";
            }


            var returnColor = new SolidColorBrush((Color)ColorConverter.ConvertFromString(fallback)!);
            if (!color.StartsWith("#")) return returnColor;
            try
            {
                returnColor = new SolidColorBrush((Color)ColorConverter.ConvertFromString(color)!);
            }
            catch (Exception)
            {
                // Failed to set color
            }
            return returnColor;
        }

        /// <summary>
        /// Shows error and exits program is program is already running
        /// </summary>
        private static void IsRunningAlready()
        {
            try
            {
                // Check if program is running, if not: return.
                if (Mutex.WaitOne(TimeSpan.Zero, true)) return;

                // The program is running at this point.
                // If set to minimize to tray, try open it.
                if (AppSettings.TrayMinimizeNotExit)
                {
                    if (NativeFuncs.BringToFront())
                        Environment.Exit(1056); // 1056	An instance of the service is already running.
                }

                // Otherwise: It has probably just closed. Wait a few and try again
                Thread.Sleep(2000); // 2 seconds before just making sure -- Might be an admin restart

                // Ignore other processes running while in DEBUG mode.
                var release = false;
#if RELEASE
release = true;
#endif
                if (Mutex.WaitOne(TimeSpan.Zero, true)) return;
                // ReSharper disable once ConditionIsAlwaysTrueOrFalse
                if (!release) return;
                // Try to show from tray, as user may not know it's hidden there.
                string text;
                if (!NativeFuncs.BringToFront())
                {
	                text = "Another TcNo Account Switcher instance has been detected." + Environment.NewLine +
	                       "[Something wrong? Hold Hold Alt, Ctrl, Shift or Scroll Lock while starting to close all TcNo processes!]";

                    _ = MessageBox.Show(text, "TcNo Account Switcher Notice", MessageBoxButton.OK,
		                MessageBoxImage.Information,
		                MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);
	                Environment.Exit(1056); // 1056	An instance of the service is already running.
                }
                else
                {
	                if (!AppSettings.ShownMinimizedNotification)
	                {
		                text = "TcNo Account Switcher was running." + Environment.NewLine +
		                       "I've brought it to the top." + Environment.NewLine +
		                       "Make sure to check your Windows Task Tray for the icon :)" + Environment.NewLine +
		                       "- You can exit it from there too" + Environment.NewLine + Environment.NewLine +
		                       "[Something wrong? Hold Alt, Ctrl, Shift or Scroll Lock while starting to close all TcNo processes!]" + Environment.NewLine + Environment.NewLine +
		                       "This message only shows once.";

                        _ = MessageBox.Show(text, "TcNo Account Switcher Notice", MessageBoxButton.OK,
			                MessageBoxImage.Information,
			                MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);

		                AppSettings.ShownMinimizedNotification = true;
		                AppSettings.SaveSettings();
	                }

	                Environment.Exit(1056); // 1056	An instance of the service is already running.
                }
            }
            catch (AbandonedMutexException)
            {
                // Just restarted
            }
        }

        /// <summary>
        /// CLI specific interface for TcNo Account Switcher
        /// (Only help commands at this moment)
        /// </summary>
        /// <param name="e">StartupEventArgs for the program</param>
        /// <returns>True if handled and should close. False if launch GUI.</returns>
        private static Task<bool> ConsoleMain(StartupEventArgs e)
        {
            Console.WriteLine(@"Welcome to the TcNo Account Switcher - Command Line Interface!");
            Console.WriteLine(@"Use -h (or --help) for more info." + Environment.NewLine);

            for (var i = 0; i != e.Args.Length; ++i)
            {
                // --- Switching to accounts ---
                if (e.Args[i]?[0] == '+')
                {
                    CliSwitch(e.Args, i);
                    continue;
                }

                // --- Switching to accounts via protocol ---
                if (e.Args[i].ToLowerInvariant().StartsWith(@"tcno:\\"))
                {
                    CliSwitch(e.Args, i);
                    continue;
                }

                // --- Log out of accounts ---
                if (e.Args[i].StartsWith("logout"))
                {
                    CliLogout(e.Args[i]);
                    continue;
                }

                switch (e.Args[i])
                {
                    case "h":
                    case "-h":
                    case "help":
                    case "--help":
                        var availableList = new List<string>();
                        var switchList = new List<string>();
                        foreach (var jToken in BasicPlatforms.GetPlatforms)
                        {
                            var line = "";
                            var x = (JProperty)jToken;
                            var identifiers = BasicPlatforms.GetPlatforms[x.Name]?["Identifiers"]?.ToObject<List<string>>();

                            if (identifiers != null)
                            {
                                switchList.Add($" -   {x.Name}: {identifiers[0]}:<identifier>");

                                foreach (var platformShort in identifiers)
                                {
                                    if (line == "")
                                        line = $" -   {x.Name}: {platformShort}";
                                    else
                                        line += $", {platformShort}";
                                }
                            }

                            availableList.Add(line);
                        }

                        availableList.Sort();
                        switchList.Sort();

                        var helpLines = new List<string>
                        {
                            "This is the command line interface. You are able to use any of the following arguments with this program:",
                            "--- Switching to accounts ---",
                            "Usage (don't include spaces): + <PlatformLetter> : <...details>",
                            " -   Steam format: +s:<steamId>[:<PersonaState (0-7)>]"
                        };
                        helpLines.AddRange(switchList);

                        helpLines.AddRange(new List<string>
                        {
                            " --- Other platforms: +<2-3 letter code>:<unique identifiers>",
                            "--- Log out of accounts ---",
                            "logout:<platform> = Logout of a specific platform (Allowing you to sign into and add a new account)",
                            "Available platforms:",
                            " -   Steam: s, steam",
                        });
                        helpLines.AddRange(availableList);
                        helpLines.AddRange(new List<string>
                        {
                            " --- Other platforms via custom commands: <unique identifiers>",
                            "--- Other arguments ---",
                            "v, vv, verbose = Verbose mode (Shows a lot of details, somewhat useful for debugging)"
                        });

                        Console.WriteLine(string.Join(Environment.NewLine, helpLines));
                        return Task.FromResult(true);
                    case "v":
                    case "vv":
                    case "verbose":
                        Globals.VerboseMode = true;
                        break;
                    default:
                        Globals.WriteToLog($"Unknown argument: \"{e.Args[i]}\"");
                        break;
                }
            }

            return Task.FromResult(true);
        }

        /// <summary>
        /// Handle account switch requests given as arguments to the CLI
        /// </summary>
        /// <param name="args">Arguments</param>
        /// <param name="i">Index of argument to process</param>
        private static void CliSwitch(string[] args, int i)
        {
            if (args.Length < i) return;
            if (args[i].StartsWith(@"tcno:\\")) // Launched through Protocol
                args[i] = '+' + args[i][7..];

            var command = args[i][1..].Split(':'); // Drop '+' and split
            var platform = command[0];
            var account = command[1];
            var remainingArguments = args[1..];
            var combinedArgs = string.Join(' ', args);

            switch (platform)
            {
                // Steam
                case "s":
                    {
                        // Steam format: +s:<steamId>[:<PersonaState (0-7)>]
                        Globals.WriteToLog("Steam switch requested");
                        if (!GeneralFuncs.CanKillProcess(TcNo_Acc_Switcher_Server.Data.Settings.Steam.Processes)) Restart(combinedArgs, true);
                        SteamSwitcherFuncs.SwapSteamAccounts(account.Split(":")[0],
                            ePersonaState: command.Length > 2
                                ? int.Parse(command[2])
                                : -1, args: string.Join(' ', remainingArguments)); // Request has a PersonaState in it
                        return;
                    }

                // BASIC ACCOUNT PLATFORM
                default:
                    if (!BasicPlatforms.PlatformExistsFromShort(platform)) break;
                    // Is a basic platform!
                    BasicPlatforms.SetCurrentPlatformFromShort(platform);
                    Globals.WriteToLog(CurrentPlatform.FullName + " switch requested");
                    if (!GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd)) Restart(combinedArgs, true);
                    BasicSwitcherFuncs.SwapBasicAccounts(account, string.Join(' ', remainingArguments));
                    break;
            }
        }

        /// <summary>
        /// Handle logout given as arguments to the CLI
        /// </summary>
        /// <param name="arg">Argument to process</param>
        private static void CliLogout(string arg)
        {
            var platform = arg.Split(':')[1];
            switch (platform.ToLowerInvariant())
            {
                // Steam
                case "s":
                case "steam":
                    Globals.WriteToLog("Steam logout requested");
                    SteamSwitcherBase.NewLogin_Steam();
                    break;

                // BASIC ACCOUNT PLATFORM
                default:
                    if (!BasicPlatforms.PlatformExistsFromShort(platform)) break;
                    // Is a basic platform!
                    BasicPlatforms.SetCurrentPlatformFromShort(platform);
                    Globals.WriteToLog(CurrentPlatform.FullName + " logout requested");
                    BasicSwitcherBase.NewLogin_Basic();
                    break;
            }
        }

        /// <summary>
        /// Moves CrashLogs and log.txt if crashed.
        /// </summary>
        public static void MoveLogs()
        {
            if (!Directory.Exists("CrashLogs")) return;
            // Create folder with current date if doesnt exist
            var todayDir = $"CrashLogs\\{DateTime.Now:dd-MM-yy}";
            if (!Directory.Exists(todayDir)) Directory.CreateDirectory(todayDir);

            // Collect all logs into one folder
            var crashFiles = Directory.EnumerateFiles("CrashLogs", "*.txt");
            foreach (var file in crashFiles)
            {
                try
                {
                    File.Move(file, Path.Join(todayDir, Path.GetFileName(file)));
                }
                catch (Exception e)
                {
                    Globals.WriteToLog(@"[Caught - MoveLogs()] crash" + e);
                }
            }

            if (crashFiles.Any())
            {
                try
                {
                    File.WriteAllText(Path.Join(todayDir, "log.txt"), Globals.ReadAllText("log.txt"));
                }
                catch (Exception e)
                {
                    Globals.WriteToLog(@"[Caught - MoveLogs()] log" + e);
                }
            }
        }

        public static string Compress(string text)
        {
            //https://www.neowin.net/forum/topic/994146-c-help-php-compatible-string-gzip/
            var buffer = Encoding.UTF8.GetBytes(text);

            var memoryStream = new MemoryStream();
            using (var gZipStream = new GZipStream(memoryStream, CompressionMode.Compress, true))
            {
                gZipStream.Write(buffer, 0, buffer.Length);
            }

            memoryStream.Position = 0;

            var compressedData = new byte[memoryStream.Length];
            _ = memoryStream.Read(compressedData, 0, compressedData.Length);

            return Convert.ToBase64String(compressedData);
        }

        #region ResizeWindows
        /*
        // https://stackoverflow.com/a/27157947/5165437
        private bool _resizeInProcess;
                private void Resize_Init(object sender, MouseButtonEventArgs e)
                {
                    if (sender is not Rectangle senderRect) return;
                    _resizeInProcess = true;
                    _ = senderRect.CaptureMouse();
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
                    if (sender is not Rectangle senderRect) return;
                    var mainWindow = senderRect.Tag as Window;
                    var width = e.GetPosition(mainWindow).X;
                    var height = e.GetPosition(mainWindow).Y;
                    _ = senderRect.CaptureMouse();
                    if (senderRect.Name.ToLowerInvariant().Contains("right"))
                    {
                        width += 5;
                        if (width > 0)
                            if (mainWindow != null)
                                mainWindow.Width = width;
                    }
                    if (senderRect.Name.ToLowerInvariant().Contains("left"))
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
                    if (senderRect.Name.ToLowerInvariant().Contains("bottom"))
                    {
                        height += 5;
                        if (height > 0)
                            if (mainWindow != null)
                                mainWindow.Height = height;
                    }

                    if (!senderRect.Name.ToLowerInvariant().Contains("top")) return;
                    height -= 5;
                    if (mainWindow == null) return;
                    mainWindow.Top += height;
                    height = mainWindow.Height - height;
                    if (height > 0)
                    {
                        mainWindow.Height = height;
                    }
                }
        */
        #endregion
    }
}
