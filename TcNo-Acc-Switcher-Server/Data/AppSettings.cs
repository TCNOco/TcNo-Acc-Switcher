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
using System.Collections.ObjectModel;
using System.Diagnostics;
using System.Drawing;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Reflection;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SharpScss;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Interfaces;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared.Toast;
using TcNo_Acc_Switcher_Server.State.Interfaces;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppSettings
    {
        [Inject] private ILang Lang { get; set; }
        [Inject] private AppData AData { get; set; }
        [Inject] private IStylesheetSettings StylesheetSettings { get; set; }

        private static AppSettings _instance = new();

        private static readonly object LockObj = new();
        public static AppSettings Instance
        {
            get
            {
                lock (LockObj)
                {
                    // Load settings if have changed, or not set
                    if (_instance is {_currentlyModifying: true}) return _instance;
                    if (_instance._lastHash != "") return _instance;

                    _instance = new AppSettings { _currentlyModifying = true };

                    if (File.Exists(SettingsFile))
                    {
                        if (File.Exists(SettingsFile)) JsonConvert.PopulateObject(File.ReadAllText(SettingsFile), _instance);
                        if (_instance == null)
                        {
                            //_ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedLoadSettings"]);
                            if (File.Exists(SettingsFile))
                                Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
                            _instance = new AppSettings { _currentlyModifying = true };
                        }
                        _instance._lastHash = Globals.GetFileMd5(SettingsFile);
                    }else
                    {
                        SaveSettings();
                    }

                    _instance.OnLoad();

                    return _instance;
                }
            }
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }

        private void OnLoad()
        {
            // if (StylesheetSettings is not null) StylesheetSettings.Load(_instance);
            CheckShortcuts();
            InitPlatformsList();
            _currentlyModifying = false;
        }


        // Constants
        private static readonly string SettingsFile = "WindowSettings.json";

        private string _lastHash = "";
        private bool _currentlyModifying;
        public static void SaveSettings() => GeneralFuncs.SaveSettings(SettingsFile, Instance);

        // Variables
        [JsonProperty(Order = 0)] public string Language = "";
        [JsonProperty(Order = 3)] public int ServerPort = 1337;
        [JsonProperty(Order = 4)] public Point WindowSize = new() { X = 800, Y = 450 };
        [JsonProperty(Order = 5)] public bool AllowTransparency = true;
        [JsonProperty("Version", Order = 6)] private readonly string _version = Globals.Version; // Just for reference in the settings JSON file
        [JsonProperty(Order = 8)] public bool TrayMinimizeNotExit;
        [JsonProperty(Order = 9)] public bool ShownMinimizedNotification;
        [JsonProperty(Order = 10)] public bool StartCentered;
        [JsonProperty(Order = 12)] public string ActiveBrowser = "WebView";
        [JsonProperty(Order = 15)] public bool ShareAnonymousStats = true;
        [JsonProperty(Order = 16)] public bool MinimizeOnSwitch;
        [JsonProperty(Order = 18)] public bool DiscordRpcShareTotalSwitches = true;
        [JsonProperty(Order = 19)] public string PasswordHash = "";
        [JsonProperty(Order = 21)] public bool AlwaysAdmin;
        [JsonIgnore] public bool DesktopShortcut;
        [JsonIgnore] public bool StartMenu;
        [JsonIgnore] public bool StartMenuPlatforms;
        [JsonIgnore] public bool ProtocolEnabled;
        [JsonIgnore] public bool TrayStartup;
        [JsonIgnore] public bool UpdateCheckRan;
        [JsonIgnore] public bool PreRenderUpdate;
        [JsonIgnore] public string PasswordCurrent;

        [JsonProperty("CollectStats", Order = 14)] private bool _statsEnabled = true;
        public bool StatsEnabled
        {
            get => _statsEnabled;
            set
            {
                if (!value) ShareAnonymousStats = false;
                _statsEnabled = value;
            }
        }

        [JsonProperty("DiscordRpcEnabled", Order = 17)] private bool _discordRpc = true;
        public bool DiscordRpc
        {
            get => _discordRpc; set
            {
                if (!value) DiscordRpcShareTotalSwitches = false;
                _discordRpc = value;
            }
        }

        #region Used in Stylesheet
        private string _activeTheme = "Dracula_Cyan";
        private bool _rtl = CultureInfo.CurrentCulture.TextInfo.IsRightToLeft;
        private string _background = "";
        private bool _streamerModeEnabled = true;
        private bool _streamerModeTriggered;

        [JsonProperty(Order = 11)]
        public string ActiveTheme
        {
            get => _activeTheme;
            set
            {
                _activeTheme = value;
                if (StylesheetSettings is null) return;
                StylesheetSettings.ActiveTheme = value;
                StylesheetSettings.NotifyDataChanged();
            }
        }

        [JsonProperty(Order = 1)]
        public bool Rtl
        {
            get => _rtl;
            set
            {
                _rtl = value;
                if (StylesheetSettings is null) return;
                StylesheetSettings.Rtl = value;
                StylesheetSettings.NotifyDataChanged();
            }
        }

        [JsonProperty(Order = 2)]
        public bool StreamerModeEnabled
        {
            get => _streamerModeEnabled;
            set
            {
                _streamerModeEnabled = value;
                if (StylesheetSettings is null) return;
                StylesheetSettings.StreamerModeEnabled = value;
                StylesheetSettings.NotifyDataChanged();
            }
        }

        [JsonProperty(Order = 13)]
        public string Background
        {
            get => _background;
            set
            {
                _background = value;
                if (StylesheetSettings is null) return;
                StylesheetSettings.BackgroundPath = value;
                StylesheetSettings.NotifyDataChanged();
            }
        }

        public bool StreamerModeTriggered
        {
            get => _streamerModeTriggered;
            set
            {
                _streamerModeTriggered = value;
                if (StylesheetSettings is null) return;
                StylesheetSettings.StreamerModeTriggered = value;
                StylesheetSettings.NotifyDataChanged();
            }
        }

        #endregion




        /// <summary>
        /// For BasicStats // Game statistics collection and showing
        /// Keys for metrics on this list are not shown for any account.
        /// List of all games:[Settings:Hidden metric] metric keys.
        /// </summary>
        [JsonProperty(Order = 20)] public Dictionary<string, Dictionary<string, bool>> GloballyHiddenMetrics = new();

        public class PlatformItem : IComparable
        {
            public string Name = "";
            [JsonIgnore] public string SafeName = "";
            [JsonIgnore] public string NameNoSpace = "";
            [JsonIgnore] public string Identifier = "";
            [JsonIgnore] public string ExeName = "";
            [JsonIgnore] public List<string> PossibleIdentifiers = new(); // Other identifiers that can refer to this platform. (b, bnet, battlenet, etc)
            public bool Enabled;
            public int DisplayIndex = 99;
            public int CompareTo(object o)
            {
                var a = this;
                var b = (PlatformItem)o;
                return string.CompareOrdinal(a.Name, b.Name);
            }

            // Needed for JSON serialization/deserialization
            public PlatformItem() { }

            // Used for first init. The rest of the info is added by BasicPlatforms.
            public PlatformItem(string name, bool enabled)
            {
                Name = name;
                SafeName = Globals.GetCleanFilePath(name);
                NameNoSpace = SafeName.Replace(" ", "");
                Enabled = enabled;
            }
            public PlatformItem(string name, List<string> identifiers, string exeName, bool enabled)
            {
                Name = name;
                SafeName = Globals.GetCleanFilePath(name);
                NameNoSpace = SafeName.Replace(" ", "");
                Enabled = enabled;
                Identifier = identifiers[0];
                PossibleIdentifiers = identifiers;
                ExeName = exeName;
            }

            /// <summary>
            /// Set from a new PlatformItem - Not including Enabled.
            /// </summary>
            public void SetFromPlatformItem(PlatformItem inItem)
            {
                Name = inItem.Name;
                SafeName = Globals.GetCleanFilePath(Name);
                NameNoSpace = SafeName.Replace(" ", "");
                Identifier = inItem.Identifier;
                ExeName = inItem.ExeName;
            }

            public void SetEnabled(bool enabled)
            {
                Enabled = enabled;
                Platforms.Sort();
            }
        }

        private static readonly ObservableCollection<PlatformItem> DefaultPlatforms = new()
        {
            new PlatformItem("Discord", true),
            new PlatformItem("Epic Games", true),
            new PlatformItem("Origin", true),
            new PlatformItem("Riot Games", true),
            new PlatformItem("Steam", true),
            new PlatformItem("Ubisoft", true),
        };

        [JsonProperty("Platforms", Order = 7)] private ObservableCollection<PlatformItem> _platforms = new();

        public static ObservableCollection<PlatformItem> Platforms
        {
            get => Instance._platforms;
            set
            {
                Instance._platforms = value;
                Instance._platforms.Sort();
            }
        }
        /// <summary>
        /// Get platform details from an identifier, or the name.
        /// </summary>
        public static PlatformItem GetPlatform(string nameOrId) => Platforms.FirstOrDefault(x => x.Name == nameOrId || x.PossibleIdentifiers.Contains(nameOrId));
        private static void InitPlatformsList()
        {
            // Add platforms, if none there.
            if (Instance._platforms.Count == 0)
                Instance._platforms = DefaultPlatforms;

            Instance._platforms.First(x => x.Name == "Steam").SetFromPlatformItem(new PlatformItem("Steam", new List<string> { "s", "steam" }, "steam.exe", true));

            // Load other platforms by initializing BasicPlatforms
            _ = BasicPlatforms.Instance;
        }

        public class GameSetting
        {
            public string SettingId { get; set; } = "";
            public bool Checked { get; set; }
        }












        /// <summary>
        /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
        /// It's basically the program's .exe file, but without ".exe".
        /// </summary>
        /// <returns>True when streaming software is running</returns>
        public bool StreamerModeCheck()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.StreamerModeCheck]");
            if (!Instance.StreamerModeEnabled) return false; // Don't hide anything if disabled.
            StreamerModeTriggered = false;
            foreach (var p in Process.GetProcesses())
            {
                switch (p.ProcessName.ToLowerInvariant())
                {
                    case "obs":
                    case "obs32":
                    case "obs64":
                    case "streamlabs obs":
                    case "wirecast":
                    case "xsplit.core":
                    case "xsplit.gamecaster":
                    case "twitchstudio":
                        StreamerModeTriggered = true;
                        Globals.WriteToLog($"Streamer mode found: {p.ProcessName}");
                        return true;
                }
            }
            return false;
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public Task<bool> GetTrayMinimizeNotExit() => Task.FromResult(TrayMinimizeNotExit);



        #region SHORTCUTS
        public void CheckShortcuts()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.CheckShortcuts]");
            DesktopShortcut = File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            StartMenu = File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            StartMenuPlatforms = Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            TrayStartup = Shortcut.StartWithWindows_Enabled();

            if (OperatingSystem.IsWindows())
                ProtocolEnabled = SharedStaticFuncs.Protocol_IsEnabled();
        }
        #endregion







        #region Updater

        /// <summary>
        /// Checks for an update
        /// </summary>
        public async void CheckForUpdate()
        {
            if (UpdateCheckRan) return;
            UpdateCheckRan = true;

            try
            {
#if DEBUG
                var latestVersion = Globals.DownloadString("https://tcno.co/Projects/AccSwitcher/api?debug&v=" + Globals.Version);
#else
                var latestVersion = Globals.DownloadString("https://tcno.co/Projects/AccSwitcher/api?v=" + Globals.Version);
#endif
                if (CheckLatest(latestVersion)) return;
                // Show notification
                try
                {
                    await AppData.InvokeVoidAsync("showUpdateBar");
                }
                catch (Exception)
                {
                    PreRenderUpdate = true;
                }
            }
            catch (Exception e) when (e is WebException or AggregateException)
            {
                if (File.Exists("WindowSettings.json"))
                {
                    try
                    {
                        var o = JObject.Parse(Globals.ReadAllText("WindowSettings.json"));
                        if (o.ContainsKey("LastUpdateCheckFail"))
                        {
                            if (!(DateTime.TryParseExact((string)o["LastUpdateCheckFail"], "yyyy-MM-dd HH:mm:ss.fff",
                                      CultureInfo.InvariantCulture, DateTimeStyles.None, out var timediff) &&
                                  DateTime.Now.Subtract(timediff).Days >= 1)) return;
                        }

                        // Has not shown error today
                        AData.ShowToastLang(ToastType.Error, "Toast_UpdateCheckFail", 15000);
                        o["LastUpdateCheckFail"] = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff");
                        await File.WriteAllTextAsync("WindowSettings.json", o.ToString());
                    }
                    catch (JsonException je)
                    {
                        Globals.WriteToLog("Could not interpret <User Data>\\WindowSettings.json.", je);
                        AData.ShowToastLang(ToastType.Error, "Toast_UserDataLoadFail", 15000);
                        File.Move("WindowSettings.json", "WindowSettings.bak.json", true);
                    }
                }
                Globals.WriteToLog(@"Could not reach https://tcno.co/ to check for updates.", e);
            }
        }

        /// <summary>
        /// Verify updater files and start update
        /// </summary>
        [JSInvokable]
        public void UpdateNow()
        {
            try
            {
                if (Globals.InstalledToProgramFiles() && !Globals.IsAdministrator || !Globals.HasFolderAccess(Globals.AppDataFolder))
                {
                    ModalData.ShowModal("confirm", ModalData.ExtraArg.RestartAsAdmin);
                    return;
                }

                Directory.SetCurrentDirectory(Globals.AppDataFolder);
                // Download latest hash list
                var hashFilePath = Path.Join(Globals.UserDataFolder, "hashes.json");
                Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/hashes.json", hashFilePath);

                // Verify updater files
                var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(Globals.ReadAllText(hashFilePath));
                if (verifyDictionary == null)
                {
                    AData.ShowToastLang(ToastType.Error, "Toast_UpdateVerifyFail");
                    return;
                }

                var updaterDict = verifyDictionary.Where(pair => pair.Key.StartsWith("updater")).ToDictionary(pair => pair.Key, pair => pair.Value);

                // Download and replace broken files
                Globals.RecursiveDelete("newUpdater", false);
                foreach (var (key, value) in updaterDict)
                {
                    if (key == null) continue;
                    if (File.Exists(key) && value == GeneralFuncs.GetFileMd5(key))
                        continue;
                    Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'), key);
                }

                AutoStartUpdaterAsAdmin();
            }
            catch (Exception e)
            {
                AData.ShowToastLang(ToastType.Error, "Toast_FailedUpdateCheck");
                Globals.WriteToLog("Failed to check for updates:" + e);
            }
            Directory.SetCurrentDirectory(Globals.UserDataFolder);
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
                    Globals.WriteToLog($"Unable to convert '{latest}' to a date and time.");
            }
            else
                Globals.WriteToLog($"Unable to convert '{latest}' to a date and time.");
            return false;
        }

        public static void StartUpdaterAsAdmin(string args = "")
        {
            var exeLocation = Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? Environment.CurrentDirectory;
            Directory.SetCurrentDirectory(exeLocation);

            var proc = new ProcessStartInfo
            {
                WorkingDirectory = exeLocation,
                FileName = "updater\\TcNo-Acc-Switcher-Updater.exe",
                Arguments = args,
                UseShellExecute = true,
                Verb = "runas"
            };

            try
            {
                _ = Process.Start(proc);
                AppData.NavigateTo("EXIT_APP", true);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                try
                {
                    AppData.NavigateTo("EXIT_APP", true);
                }
                catch (Exception e)
                {
                    Globals.WriteToLog("Could not close application... Just ending the server." + Environment.NewLine + e);
                    Environment.Exit(0);
                }
            }
        }

        public void AutoStartUpdaterAsAdmin(string args = "")
        {
            // Run updater
            if (Globals.InstalledToProgramFiles() || !Globals.HasFolderAccess(Globals.AppDataFolder))
            {
                StartUpdaterAsAdmin(args);
            }
            else
            {
                _ = Process.Start(new ProcessStartInfo(Path.Join(Globals.AppDataFolder, @"updater\\TcNo-Acc-Switcher-Updater.exe")) { UseShellExecute = true, Arguments = args });
                try
                {
                    AppData.NavigateTo("EXIT_APP", true);
                }
                catch (NullReferenceException)
                {
                    Environment.Exit(0);
                }
            }
        }
        #endregion
    }
}
