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
using System.Globalization;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Forms;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Shapes;
using Microsoft.VisualBasic;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.General;
using static TcNo_Acc_Switcher_Client.MainWindow;
using AppSettings = TcNo_Acc_Switcher_Server.Data.AppSettings;
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
#pragma warning disable CA2211 // Non-constant fields should not be visible - Accessed from App.xaml.cs
        public static string StartPage = "";
#pragma warning restore CA2211 // Non-constant fields should not be visible
        private static readonly HttpClient Client = new();

        internal static class NativeMethods
        {
            // http://msdn.microsoft.com/en-us/library/ms681944(VS.85).aspx
            // See: http://www.codeproject.com/tips/68979/Attaching-a-Console-to-a-WinForms-application.aspx
            // And: https://stackoverflow.com/questions/2669463/console-writeline-does-not-show-up-in-output-window/2669596#2669596
            [DllImport("kernel32.dll", SetLastError = true)]
            internal static extern int AllocConsole();

            [DllImport("kernel32.dll", SetLastError = true)]
            internal static extern int FreeConsole();

            [DllImport("kernel32.dll", SetLastError = true)]
            private static extern IntPtr GetConsoleWindow();

            [DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
            private static extern bool SetWindowText(IntPtr hwnd, string lpString);

            [DllImport("kernel32.dll", SetLastError = true)]
            private static extern bool AttachConsole(int dwProcessId);

            public static void SetWindowText(string text)
            {
                var handle = GetConsoleWindow();

                _ = SetWindowText(handle, text);
            }

            public static bool AttachToConsole(int dwProcessId) => AttachConsole(dwProcessId);
        }

        protected override void OnExit(ExitEventArgs e)
        {
            _ = NativeMethods.FreeConsole();
            Mutex.ReleaseMutex();
        }

        private static readonly Mutex Mutex = new(true, "{A240C23D-6F45-4E92-9979-11E6CE10A22C}");

        [STAThread]
        protected override async void OnStartup(StartupEventArgs e)
        {
            // Ensure files in documents are available.
            Globals.CreateDataFolder(false);

            Directory.SetCurrentDirectory(Globals.UserDataFolder);

            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
            // Upload crash logs if any, before starting program
            UploadLogs();

            if (e.Args.Length != 0) // An argument was passed
            {
                // Iterate through arguments
                foreach (var eArg in e.Args)
                {
                    // Check if arguments are a platform
                    foreach (var platform in Globals.PlatformList)
                    {
                        // Is a platform -- In other words: Show that platform.
                        if (string.Equals(platform, eArg, StringComparison.InvariantCultureIgnoreCase))
                            StartPage = platform;
                    }

                    // Check if it was verbose mode.
                    Globals.VerboseMode = Globals.VerboseMode || eArg is "v" or "vv" or "verbose";
                }


                // Was not asked to open a platform screen, or was NOT just the verbose mode argument
                // - Therefore: It was a CLI command
                // - But, do check if the tcno:\\ cli command was issued, and only that - If it was: start the program.
                if (StartPage == "" && !(Globals.VerboseMode && e.Args.Length == 1) && !(e.Args.Length == 1 && (e.Args[0] == @"tcno:\\" || e.Args[0] == "tcno:\\")))
                {
                    if (!NativeMethods.AttachToConsole(-1)) // Attach to a parent process console (ATTACH_PARENT_PROCESS)
                        _ = NativeMethods.AllocConsole();
                    Console.WriteLine(Environment.NewLine);
                    var shouldClose = await ConsoleMain(e).ConfigureAwait(false);
                    if (shouldClose)
                    {
                        Console.WriteLine(Environment.NewLine + @"Press any key to close this window...");
                        _ = NativeMethods.FreeConsole();
                        Environment.Exit(0);
                        return;
                    }
                    else
                    {
                        _ = NativeMethods.FreeConsole();
                    }
                }

            }

#if DEBUG
            _ = NativeMethods.AllocConsole();
            NativeMethods.SetWindowText("Debug console");
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
                    GeneralFuncs.RecursiveDelete(new DirectoryInfo("updater"), false);
                }
                catch (IOException)
                {
                    // Catch first IOException and try to kill the updater, if it's running... Then continue.
                    Globals.KillProcess("TcNo-Acc-Switcher-Updater");
                    GeneralFuncs.RecursiveDelete(new DirectoryInfo("updater"), false);
                }

                Directory.Move("newUpdater", "updater");
            }

            // Clear WebView2 cache
            Globals.ClearWebCache();

            // Check for update in another thread
            new Thread(CheckForUpdate).Start();

            // Show window (Because no command line commands were parsed)
            var mainWindow = new MainWindow();
            mainWindow.Show();

            if (File.Exists("LastError.txt"))
            {
                var lastError = await File.ReadAllLinesAsync("LastError.txt");
                lastError = lastError.Skip(1).ToArray();
                // TODO: Work in progress:
                //ShowErrorMessage("Error from last crash", "Last error message:" + Environment.NewLine + string.Join(Environment.NewLine, lastError));
                MessageBox.Show("Last error message:" + Environment.NewLine + string.Join(Environment.NewLine, lastError), "Error from last crash", MessageBoxButton.OK, MessageBoxImage.Error, MessageBoxResult.OK);
                File.Delete("LastError.txt");
            }
        }

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

        public static SolidColorBrush GetStylesheetColor(string key, string fallback)
        {
            string color;
            try
            {
                var start = AppSettings.Instance.Stylesheet.IndexOf(key + ":", StringComparison.Ordinal) + key.Length + 1;
                var end = AppSettings.Instance.Stylesheet.IndexOf(";", start, StringComparison.Ordinal);
                color = AppSettings.Instance.Stylesheet.Substring(start, end - start);
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

            if (!AppSettings.Instance.LoadFromFile())
            {
                if (File.Exists("StyleSettings_ErrorInfo.txt"))
                {
                    var errorText = Globals.ReadAllText("StyleSettings_ErrorInfo.txt").Split("\n");
                    _ = MessageBox.Show(
                        "Could not load StyleSettings.json! Error details: " + Environment.NewLine +
                        errorText[0] + Environment.NewLine + Environment.NewLine +
                        "The default file will be loaded." + Environment.NewLine +
                        "See \"StyleSettings_broken.yaml\" for the broken style settings." + Environment.NewLine +
                        "See \"StyleSettings_ErrorInfo.txt\" for the full error message. Above is only the first line.", "Failed to load Styles", MessageBoxButton.OK,
                        MessageBoxImage.Error);
                    try
                    {
                        _ = AppSettings.Instance.LoadFromFile();
                    }
                    catch (Exception e)
                    {
                        _ = MessageBox.Show(
                            "Another unexpected error occurred! Error details: " + Environment.NewLine +
                            e, "Failed to load Styles (2)", MessageBoxButton.OK, MessageBoxImage.Error);
                        throw;
                    }
                }
            }
            try
            {
                // Check if program is running, if not: return.
                if (Mutex.WaitOne(TimeSpan.Zero, true)) return;

                // The program is running at this point.
                // If set to minimize to tray, try open it.
                if (AppSettings.Instance.TrayMinimizeNotExit)
                {
                    if (Globals.BringToFront())
                        Environment.Exit(1056); // 1056	An instance of the service is already running.
                }

                // Otherwise: It has probably just closed. Wait a few and try again
                Thread.Sleep(2000); // 2 seconds before just making sure -- Might be an admin restart

                if (Mutex.WaitOne(TimeSpan.Zero, true)) return;
                // Try to show from tray, as user may not know it's hidden there.
                string text;
                if (!Globals.BringToFront())
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
	                if (!AppSettings.Instance.ShownMinimizedNotification)
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

		                AppSettings.Instance.ShownMinimizedNotification = true;
		                AppSettings.Instance.SaveSettings();
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
        private static async Task<bool> ConsoleMain(StartupEventArgs e)
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
                    await CliLogout(e.Args[i]).ConfigureAwait(false);
                    continue;
                }

                switch (e.Args[i])
                {
                    case "h":
                    case "-h":
                    case "help":
                    case "--help":
                        string[] help =
                        {
                            "This is the command line interface. You are able to use any of the following arguments with this program:",
                            "--- Switching to accounts ---",
                            "Usage (don't include spaces): + <PlatformLetter> : <...details>",
                            " -   Battlenet format: +b:<email>",
                            " -   Discord format: +d:<username>",
                            " -   Epic Games format: +e:<username>",
                            " -   Origin format: +o:<accName>[:<State (10 = Offline/0 = Default)>]",
                            " -   Riot Games format: +e:<username>",
                            " -   Steam format: +s:<steamId>[:<PersonaState (0-7)>]",
                            " -   Ubisoft Connect format: +u:<email>[:<0 = Online/1 = Offline>]",
                            "--- Log out of accounts ---",
                            "logout:<platform> = Logout of a specific platform (Allowing you to sign into and add a new account)",
                            "Available platforms:",
                            " -   BattleNet: b, bnet, battlenet, blizzard",
                            " -   Epic Games: e, epic, epicgames",
                            " -   Origin: o, origin, ea",
                            " -   Riot Games: r, riot, riotgames",
                            " -   Steam: s, steam",
                            " -   Ubisoft Connect: u, ubi, ubisoft, ubisoftconnect, uplay",
                            "--- Other arguments ---",
                            "v, vv, verbose = Verbose mode (Shows a lot of details, somewhat useful for debugging)"
                        };
                        Console.WriteLine(string.Join(Environment.NewLine, help));
                        return true;
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

            return true;
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

            switch (platform[^1..])
            {
                // Battle.Net
                case "b":
                    {
                        // Battlenet format: +b:<email>
                        Globals.WriteToLog("Battle.net switch requested");
                        if (!GeneralFuncs.CanKillProcess("Battle.net"))
                            RestartAsAdmin(combinedArgs);
                        BattleNet.Instance.LoadFromFile();
                        _ = TcNo_Acc_Switcher_Server.Pages.BattleNet.BattleNetSwitcherFuncs.SwapBattleNetAccounts(account, string.Join(' ', remainingArguments));
                        return;
                    }
                // Discord
                case "d":
                    {
                        // Discord format: +d:<username>
                        Globals.WriteToLog("Discord switch requested");
                        string pass;

                        if (command.Length < 3) // Get password as third argument, or as input from user.
                        {
                            while (true)
                            {
                                // Get password from user
                                Globals.WriteToLog("Please insert your Discord account switcher password:");
                                pass = CliGetPass();
                                Globals.WriteToLog(Environment.NewLine);

                                if (string.IsNullOrEmpty(pass))
                                {
                                    Globals.WriteToLog("Error: Password is required to decrypt data, to switch accounts.");
                                    Console.WriteLine(Environment.NewLine + @"Press any key to close this window...");
                                    return;
                                }

                                if (!GeneralInvocableFuncs.GiVerifyPlatformPassword("Discord", pass))
                                    Globals.WriteToLog("Error: Password is incorrect." + Environment.NewLine);
                                else
                                    break;
                            }
                        }
                        else
                        {
                            pass = command[2];
                            if (!GeneralInvocableFuncs.GiVerifyPlatformPassword("Discord", pass))
                            {
                                Globals.WriteToLog("Error: Password is incorrect.");
                                Console.WriteLine(Environment.NewLine + @"Press any key to close this window...");
                                return;
                            }
                        }

                        if (!GeneralFuncs.CanKillProcess("Discord"))
                            RestartAsAdmin(combinedArgs);
                        Discord.Instance.LoadFromFile();
                        TcNo_Acc_Switcher_Server.Pages.Discord.DiscordSwitcherFuncs.SwapDiscordAccounts(account, string.Join(' ', remainingArguments));

                        return;
                    }
                // Epic Games
                case "e":
                    {
                        // Epic Games format: +e:<username>
                        Globals.WriteToLog("Epic Games switch requested");
                        if (!GeneralFuncs.CanKillProcess("EpicGamesLauncher.exe")) RestartAsAdmin(combinedArgs);
                        Epic.Instance.LoadFromFile();
                        TcNo_Acc_Switcher_Server.Pages.Epic.EpicSwitcherFuncs.SwapEpicAccounts(account, string.Join(' ', remainingArguments));
                        return;
                    }
                // Origin
                case "o":
                    {
                        // Origin format: +o:<accName>[:<State (10 = Offline/0 = Default)>]
                        Globals.WriteToLog("Origin switch requested");
                        if (!GeneralFuncs.CanKillProcess("Origin")) RestartAsAdmin(combinedArgs);
                        Origin.Instance.LoadFromFile();
                        TcNo_Acc_Switcher_Server.Pages.Origin.OriginSwitcherFuncs.SwapOriginAccounts(account,
                            command.Length > 2 ? int.Parse(command[2]) : 0, string.Join(' ', remainingArguments));
                        return;
                    }
                // Riot Games
                case "r":
                    {
                        // Riot Games format: +e:<username>
                        Globals.WriteToLog("Riot Games switch requested");
                        if (!TcNo_Acc_Switcher_Server.Pages.Riot.RiotSwitcherFuncs.CanCloseRiot()) RestartAsAdmin(combinedArgs);
                        Riot.Instance.LoadFromFile();
                        TcNo_Acc_Switcher_Server.Pages.Riot.RiotSwitcherFuncs.SwapRiotAccounts(account.Replace('-', '#'), string.Join(' ', remainingArguments));
                        return;
                    }
                // Steam
                case "s":
                    {
                        // Steam format: +s:<steamId>[:<PersonaState (0-7)>]
                        Globals.WriteToLog("Steam switch requested");
                        if (!GeneralFuncs.CanKillProcess("steam")) RestartAsAdmin(combinedArgs);
                        Steam.Instance.LoadFromFile();
                        TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts(account.Split(":")[0],
                            ePersonaState: command.Length > 2
                                ? int.Parse(command[2])
                                : -1, args: string.Join(' ', remainingArguments)); // Request has a PersonaState in it
                        return;
                    }
                // Ubisoft
                case "u":
                    {
                        // Ubisoft Connect format: +u:<email>[:<0 = Online/1 = Offline>]
                        Globals.WriteToLog("Ubisoft Connect switch requested");
                        if (!GeneralFuncs.CanKillProcess("upc")) RestartAsAdmin(combinedArgs);
                        Ubisoft.Instance.LoadFromFile();
                        TcNo_Acc_Switcher_Server.Pages.Ubisoft.UbisoftSwitcherFuncs.SwapUbisoftAccounts(account,
                            command.Length > 2 ? int.Parse(command[2]) : -1, string.Join(' ', remainingArguments));
                        break;
                    }
            }
        }

        /// <summary>
        /// Gets a user to enter a string, that is shown as asterisks on the screen.
        /// </summary>
        /// <returns>String entered by user</returns>
        private static string CliGetPass()
        {
            var pass = string.Empty;
            ConsoleKey key;
            do
            {
                var keyInfo = Console.ReadKey(intercept: true);
                key = keyInfo.Key;

                if (key == ConsoleKey.Backspace && pass.Length > 0)
                {
                    Console.Write("\b \b");
                    pass = pass[0..^1];
                }
                else if (!char.IsControl(keyInfo.KeyChar))
                {
                    Console.Write("*");
                    pass += keyInfo.KeyChar;
                }
            } while (key != ConsoleKey.Enter);

            return pass;
        }

        /// <summary>
        /// Handle logouts given as arguments to the CLI
        /// </summary>
        /// <param name="arg">Argument to process</param>
        private static async Task CliLogout(string arg)
        {
            var platform = arg.Split(':')[1];
            switch (platform.ToLowerInvariant())
            {
                // Battle.net
                case "b":
                case "bnet":
                case "battlenet":
                case "blizzard":
                    Globals.WriteToLog("Battle.net logout requested");
                    await TcNo_Acc_Switcher_Server.Pages.BattleNet.BattleNetSwitcherBase.NewLogin_BattleNet();
                    break;

                // Discord
                case "d":
                case "discord":
                    Globals.WriteToLog("Discord logout requested");
                    TcNo_Acc_Switcher_Server.Pages.Discord.DiscordSwitcherBase.NewLogin_Discord();
                    break;

                // Epic Games
                case "e":
                case "epic":
                case "epicgames":
                    Globals.WriteToLog("Epic Games logout requested");
                    TcNo_Acc_Switcher_Server.Pages.Epic.EpicSwitcherBase.NewLogin_Epic();
                    break;

                // Origin
                case "o":
                case "origin":
                case "ea":
                    Globals.WriteToLog("Origin logout requested");
                    TcNo_Acc_Switcher_Server.Pages.Origin.OriginSwitcherBase.NewLogin_Origin();
                    break;

                // Riot Games
                case "r":
                case "riot":
                case "riotgames":
                    Globals.WriteToLog("Riot Games logout requested");
                    TcNo_Acc_Switcher_Server.Pages.Riot.RiotSwitcherBase.NewLogin_Riot();
                    break;
                // Steam
                case "s":
                case "steam":
                    Globals.WriteToLog("Steam logout requested");
                    TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherBase.NewLogin_Steam();
                    break;

                // Ubisoft Connect
                case "u":
                case "ubi":
                case "ubisoft":
                case "ubisoftconnect":
                case "uplay":
                    Globals.WriteToLog("Ubisoft Connect logout requested");
                    TcNo_Acc_Switcher_Server.Pages.Ubisoft.UbisoftSwitcherBase.NewLogin_Ubisoft();
                    break;
            }
        }

        /// <summary>
        /// Uploads CrashLogs and log.txt if crashed.
        /// </summary>
        public static void UploadLogs()
        {
            if (!Directory.Exists("CrashLogs")) return;
            if (!Directory.Exists("CrashLogs\\Submitted")) _ = Directory.CreateDirectory("CrashLogs\\Submitted");

            // Collect all logs into one string to compress
            var postData = new Dictionary<string, string>();
            var combinedCrashLogs = "";
            foreach (var file in Directory.EnumerateFiles("CrashLogs", "*.txt"))
            {
                try
                {
                    combinedCrashLogs += Globals.ReadAllText(file);
                    File.Move(file, $"CrashLogs\\Submitted\\{Path.GetFileName(file)}");
                }
                catch (Exception e)
                {
                    Globals.WriteToLog(@"[Caught - UploadLogs()]" + e);
                }
            }

            // If no logs collected, return.
            if (combinedCrashLogs == "") return;

            // Else: send log file as well.
            if (File.Exists("log.txt"))
            {
                try
                {
                    postData.Add("logs", Compress(Globals.ReadAllText("log.txt")));

                }
                catch (Exception e)
                {
                    Globals.WriteToLog(@"[Caught - UploadLogs()]" + e);
                }
            }

            // Send report to server
            postData.Add("crashLogs", Compress(combinedCrashLogs));
            if (postData.Count == 0) return;

            try
            {
                HttpContent content = new FormUrlEncodedContent(postData);
                _ = Client.PostAsync("https://tcno.co/Projects/AccSwitcher/api/crash/index.php", content);
            }
            catch (Exception e)
            {
                File.WriteAllText($"CrashLogs\\CrashLogUploadErr-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt", e.ToString());
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

        /// <summary>
        /// Checks for an update
        /// </summary>
        /// <returns>True if update found, show notification</returns>
        public static void CheckForUpdate()
        {
            try
            {
#if DEBUG
                var latestVersion = new WebClient().DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?debug&v=" + Globals.Version));
#else
                var latestVersion = new WebClient().DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?v=" + Globals.Version));
#endif
                if (CheckLatest(latestVersion)) return;
                // Show notification
                AppSettings.Instance.UpdateAvailable = true;
            }
            catch (WebException e)
            {
                if (File.Exists("WindowSettings.json"))
                {
                    var o = JObject.Parse(Globals.ReadAllText("WindowSettings.json"));
                    if (o.ContainsKey("LastUpdateCheckFail"))
                    {
                        if (!(DateTime.TryParseExact((string)o["LastUpdateCheckFail"], "yyyy-MM-dd HH:mm:ss.fff",
                                  CultureInfo.InvariantCulture, DateTimeStyles.None, out var timediff) &&
                              DateTime.Now.Subtract(timediff).Days >= 1)) return;
                    }
                    // Has not shown error today
                    MessageBox.Show("Could not reach https://tcno.co/ to check for updates. [This message will show once a day]", "Unable to check for updates", MessageBoxButton.OK, MessageBoxImage.Error);
                    o["LastUpdateCheckFail"] = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff");
                    File.WriteAllText("WindowSettings.json", o.ToString());
                }
                Globals.WriteToLog(@"Could not reach https://tcno.co/ to check for updates.\n" + e);
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
                if (DateTime.TryParseExact(Globals.Version, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var currentDate))
                {
                    if (latestDate.Equals(currentDate) || currentDate.Subtract(latestDate) > TimeSpan.Zero) return true;
                }
                else
                    Globals.WriteToLog(@$"Unable to convert '{0}' to a date and time. {latest}");
            }
            else
                Globals.WriteToLog(@$"Unable to convert '{0}' to a date and time. {latest}");
            return false;
        }


        #region ResizeWindows
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
        #endregion
    }
}
