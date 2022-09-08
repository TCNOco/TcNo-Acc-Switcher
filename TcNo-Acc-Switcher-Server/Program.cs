// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
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
using System.CommandLine;
using System.CommandLine.Binding;
using System.CommandLine.Builder;
using System.CommandLine.Parsing;
using System.IO;
using System.Linq;
using System.Net.NetworkInformation;
using System.Text.RegularExpressions;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server;

public class Program
{
    private static IWindowSettings _windowSettings;
    private static JArray _platformsArray;
    public static void Main(string[] args)
    {
        // TODO: Add crash handler.

        // Create WindowSettings instance. This loads from saved file if available.
        _windowSettings = new WindowSettings();
        LoadPlatforms();

        // TODO: Convert "+s:<steamId>[:<PersonaState (0-7)>]", "+s", "+<2-3 letter code>:<unique identifiers>", etc to arguments and process. Then use that below instead of the original args. (If any contain "+")
        // TODO: "logout:<platform>"
        // TODO: "tcno:\\"

        #region region Server settings
        var url = new Option<string>(
            new[] {"-u", "--url"},
            () => ConfigSettingsDefaults.Url,
            "Specify the URL for the server [MUST include http://].");

        var port = new Option<int>(
            new[] { "--port" },
            () => ConfigSettingsDefaults.Port,
            "Specify the port for the web server/application.");

        var noBrowser = new Option<bool>(
            new[] { "-nb", "--nobrowser" },
            () => false,
            "When set the system browser window will not open to the TcNo Account Switcher GUI.");

        var listPlatforms = new Option<bool>(
            new [] { "--list", "--list-platforms" },
            "List all platforms available, as below.");

        var verbose = new Option<bool>(
            new [] { "-vv", "--verbose" },
            "Display lots of extra information in console for debugging.");
        #endregion
        #region Platform actions
        var platform = new Option<string>(
            new[] {"-p", "--platform"},
            "When only this is set: Open the switcher GUI to this page. Else, if account ID set: Swap to that account or perform another action.");

        var accountId = new Option<string>(
            new[] {"-id", "--id"},
            "The account ID to swap to, or perform another action on. Needs `-p` or `--platform` to be set.");

        var accountState = new Option<int>(
            "--state",
            () => ConfigSettingsDefaults.AccountState,
            "Set the profile state for the requested account. Specific platforms only (Only Steam).");

        var logout = new Option<bool>(
            new[] {"-l", "--logout"},
            "Log out of the account. Needs `-p` or `--platform` to be set.");
        #endregion
        var rootCommand = new RootCommand(
            "The TcNo Account Switcher is a Super-fast account switcher for Steam, Battle.net, Epic Games, Origin, Riot, Ubisoft and many others! https://github.com/TcNobo/TcNo-Acc-Switcher/" + Environment.NewLine +
            "Welcome to the TcNo Account Switcher - Command Line Interface!" + Environment.NewLine +
            "Use -h (or --help) for more info.")
        {
            url,
            port,
            noBrowser,
            listPlatforms, // This value is never used, other than displaying below by checking argument list directly.
            verbose,
            platform,
            accountId,
            accountState,
            logout
        };
        rootCommand.SetHandler(LoadConfig, url, port, noBrowser, verbose, platform, accountId, accountState, logout);

        // Run with a console command, and should show console window (if not already).
        var shouldHaveConsoleIf = new[] { "-nb", "--nobrowser", "--list", "--list-platforms", "-vv", "--verbose" };
        if (args.Any(arg => shouldHaveConsoleIf.Any(s => arg.Equals(s, StringComparison.InvariantCultureIgnoreCase)) || arg.StartsWith("tcno:\\", StringComparison.InvariantCultureIgnoreCase)))
        {
            if (!NativeFuncs.AttachToConsole(-1)) // Attach to a parent process console (ATTACH_PARENT_PROCESS)
                _ = NativeFuncs.AllocConsole();

            // If necessary, use _ = NativeFuncs.FreeConsole(); to remove the console later.
            // TODO: This needs to be tested, but can't until the main Client is compatible.
        }

        var commandLineBuilder = new CommandLineBuilder(rootCommand).UseDefaults();
        var parser = commandLineBuilder.Build();
        parser.Invoke(args);

        // If help command was passed, display list of platforms as well. Then close.
        var helpArgs = new[] { "-?", "-h", "--help", "--list", "--list-platforms" };
        if (args.Any(arg => helpArgs.Any(noRunArg => arg.Equals(noRunArg, StringComparison.InvariantCultureIgnoreCase))))
        {
            // Foreach platform in platformsArray, get Name and Identifiers[]
            foreach (var p in _platformsArray)
            {
                var platformName = p["Name"]?.ToString();
                var platformIdentifiers = p["Identifiers"]?.Select(identifier => identifier.ToString()).ToList();
                if (string.IsNullOrEmpty(platformName) || platformIdentifiers == null || platformIdentifiers.Count == 0) continue;
                Console.WriteLine($@" - {platformName} ({string.Join(", ", platformIdentifiers)})");
            }
            return;
        }

        // Don't open server if specific commands were passed.
        var noRunArgs = new[] { "-?", "-h", "--help" };
        // Check if any items in args are in noRunArgs, ignoring case
        if (args.Any(arg => noRunArgs.Any(noRunArg => arg.Equals(noRunArg, StringComparison.OrdinalIgnoreCase))))
            return;

        if (Config.HasPlatform && Config.Logout)
        {
            // TODO: Logout of platform here.
            return;
        }

        if (Config.HasPlatform && Config.HasAccountId)
        {
            // TODO: Switch to another account here.
            // Also check Config.HasAccountState
            return;
        }


        // Set first page
        Config.SetLandingPage();

        // Upload crash logs if any, before starting program
        Globals.UploadLogs();

        // Run the main server
        MainProgram();
    }

    private static void LoadPlatforms() {
        // Read Platforms.json as JArray
        var platforms = Path.Join(Globals.AppDataFolder, "Platforms.json");
        var platformsJson = File.ReadAllText(platforms);
        _platformsArray = JsonConvert.DeserializeObject<JArray>(platformsJson);

        if (_platformsArray == null) return;
        Console.WriteLine("Available platforms: ");
        if (_platformsArray.All(x => x["Name"]?.ToString() != "Steam"))
        {
            // Steam is not in array.
            var steam = new JObject
            {
                {"Identifiers", JArray.FromObject(new[] {"s", "steam"})},
                {"Name", "Steam"}
            };
            _platformsArray.Add(steam);
        }
        // Sort platformsArray by Name
        _platformsArray = new JArray(_platformsArray.OrderBy(x => x["Name"]?.ToString()));
    }
    private static class ConfigSettingsDefaults
    {
        public const string Url = "http://localhost";
        public const int Port = 0;
        public const int AccountState = -1;
        public const string LandingPage = "/";
    }

    public class ConfigSettings
    {
        public string Url;
        public int Port;
        public bool NoBrowser;

        private string _platform;
        public string Platform { get => _platform; set => _platform = ExpandStaticPlatforms(value); }
        public bool HasPlatform => string.IsNullOrEmpty(Platform);

        public string AccountId;
        public bool HasAccountId => string.IsNullOrEmpty(AccountId);

        public int AccountState;
        public bool HasAccountState => AccountState != ConfigSettingsDefaults.AccountState;

        public string LandingPage = ConfigSettingsDefaults.LandingPage;
        public bool Logout;

        public string GetRootUrl => $"{Url}:{Port}";
        public string GetCompleteUrl => $"{GetRootUrl}{LandingPage}";
        public bool IsLandingPageDefault => LandingPage == ConfigSettingsDefaults.LandingPage;
        public void ResetLandingPage() => LandingPage = ConfigSettingsDefaults.LandingPage;

        /// <summary>
        /// When Platform is set, but not AccountId (No action to be performed) -> Redirect user from the landing page to the platform page.
        /// </summary>
        public void SetLandingPage()
        {
            if (string.IsNullOrEmpty(Platform)) return;
            // First platform from _platformsArray where Identifiers contains Platform or Name is Platform
            var platform = _platformsArray.FirstOrDefault(x => x["Identifiers"]?.Select(identifier => identifier.ToString()).Contains(Platform) ?? false || x["Name"]?.ToString() == Platform);

            if (Platform == "Steam")
            {
                LandingPage = "/Steam";
                return;
            }

            if (platform is null) return;
            var platformName = platform["Identifiers"]?[0]?.ToString();
            if (string.IsNullOrEmpty(platformName)) return;
            LandingPage = "/" + platformName;
        }

        private string ExpandStaticPlatforms(string id)
        {
            var lowerInvariant = id.ToLowerInvariant();
            if (lowerInvariant == "s") return "Steam";
            return id;
        }
    }
    public static readonly ConfigSettings Config = new();


    private static void LoadConfig(string url, int port, bool noBrowser, bool verbose, string platform, string accountId, int accountState, bool logout)
    {
        Config.Url = url;
        Config.Port = port;
        Config.NoBrowser = noBrowser;
        Globals.VerboseMode = verbose;
        Config.Platform = platform;
        Config.AccountId = accountId;
        Config.AccountState = accountState;
        Config.Logout = logout;
    }


    /*

    /// <summary>
    /// Handle logout given as arguments to the CLI
    /// </summary>
    /// <param name="arg">Argument to process</param>
    private static async Task CliLogout(string arg)
    {
        var platform = arg.Split(':')[1];
        switch (platform.ToLowerInvariant())
        {
            // Steam
            case "s":
            case "steam":
                Globals.WriteToLog("Steam logout requested");
                AppState.Switcher.CurrentSwitcher = "Steam";
                await AppFuncs.SwapToNewAccount();
                break;

            // BASIC ACCOUNT PLATFORM
            default:
                if (WindowSettings.GetPlatform(platform) is null) break;
                // Is a basic platform!
                BasicPlatforms.SetCurrentPlatform(platform);
                Globals.WriteToLog(CurrentPlatform.FullName + " logout requested");
                AppState.Switcher.CurrentSwitcher = platform;
                await AppFuncs.SwapToNewAccount();
                break;
        }
    }



    /// <summary>
    /// Handle account switch requests given as arguments to the CLI
    /// </summary>
    /// <param name="args">Arguments</param>
    /// <param name="i">Index of argument to process</param>
    private static async Task CliSwitch(string[] args, int i)
    {
        if (args.Length < i) return;
        if (args[i].StartsWith(@"tcno:\\")) // Launched through Protocol
            args[i] = '+' + args[i][7..];

        var command = args[i][1..].Split(':'); // Drop '+' and split
        var platform = command[0];
        var account = command[1];
        var remainingArguments = args[1..];
        var combinedArgs = string.Join(' ', args);

        if (platform == "s")
        {
            // Steam format: +s:<steamId>[:<PersonaState (0-7)>]
            Globals.WriteToLog("Steam switch requested");
            if (!GeneralFuncs.CanKillProcess(TcNo_Acc_Switcher_Server.Data.Settings.Steam.Processes)) Restart(combinedArgs, true);
            await SteamSwitcherFuncs.SwapSteamAccounts(account.Split(":")[0],
                ePersonaState: command.Length > 2
                    ? int.Parse(command[2])
                    : -1, args: string.Join(' ', remainingArguments)); // Request has a PersonaState in it
            return;
        }

        if (WindowSettings.GetPlatform(platform) is null) return;
        BasicPlatforms.SetCurrentPlatform(platform);
        Globals.WriteToLog(CurrentPlatform.FullName + " switch requested");
        if (!GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd)) Restart(combinedArgs, true);
        BasicSwitcherFuncs.SwapTemplatedAccounts(account, string.Join(' ', remainingArguments));
    }

    */

    public static void MainProgram()
    {
        Console.WriteLine(Config.ToString());

        // Set working directory to documents folder
        Globals.CreateDataFolder(false);
        Directory.SetCurrentDirectory(Globals.UserDataFolder);

        try
        {
            CreateHostBuilder().Build().Run();
        }
        catch (IOException ioe)
        {
            // Failed to bind to port
            if (ioe.HResult != -2146232800) throw;
            Globals.WriteToLog(ioe);
        }
    }

    private static IHostBuilder CreateHostBuilder()
    {
        // Check if is admin, and if needs to be restarted as admin.
        if (_windowSettings.AlwaysAdmin && !Globals.IsAdministrator) StaticFuncs.RestartAsAdmin();

        if (Config.Port == ConfigSettingsDefaults.Port)
        {
            // The server was launched without the client, or a specified port
            Config.Port = FindOpenPort(_windowSettings.ServerPort);
            Console.WriteLine(@"Using saved/random port: " + Config.Port);
        }

        // Save new port, if different.
        if (_windowSettings.ServerPort != Config.Port)
            _windowSettings.Save();

        // Start browser - if not started with nobrowser
        if (!Config.NoBrowser)
        {
            Config.ResetLandingPage();
            StaticFuncs.OpenLinkInBrowser(Config.GetCompleteUrl);
        }

        return Host.CreateDefaultBuilder()
            .ConfigureWebHostDefaults(webBuilder =>
            {
                _ = webBuilder.UseStartup<Startup>()
                    .UseUrls(Config.GetRootUrl);
            });
    }

    /// <summary>
    /// Find first available port. Returns open port to use.
    /// May return same as input, if it was open.
    /// </summary>
    public static int FindOpenPort(int port = ConfigSettingsDefaults.Port)
    {
        Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.FindOpenPort]");
        // Pick random port if none assigned.
        if (port == ConfigSettingsDefaults.Port) port = NewPort();

        // Check if port available:
        var ipGlobalProperties = IPGlobalProperties.GetIPGlobalProperties();
        var tcpConnInfoArray = ipGlobalProperties.GetActiveTcpConnections();
        while (true)
        {
            if (tcpConnInfoArray.All(x => x.LocalEndPoint.Port != port)) break;
            port = NewPort();
        }

        return port;
    }

    private static int NewPort()
    {
        var r = new Random();
        return r.Next(20000, 40000); // Random int [Why this range? See: https://www.sciencedirect.com/topics/computer-science/registered-port & netsh interface ipv4 show excludedportrange protocol=tcp]
    }
}