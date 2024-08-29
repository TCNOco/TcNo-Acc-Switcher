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
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Steam
    {
        private static readonly Lang Lang = Lang.Instance;
        private static Steam _instance = new();
        private static readonly object LockObj = new();
        public static Steam Instance
        {
            get
            {
                lock (LockObj)
                {
                    // Load settings if have changed, or not set
                    if (_instance is { _currentlyModifying: true }) return _instance;
                    if (_instance != new Steam() && Globals.GetFileMd5(SettingsFile) == _instance._lastHash) return _instance;

                    _instance = new Steam { _currentlyModifying = true };

                    if (File.Exists(SettingsFile))
                    {
                        _instance = JsonConvert.DeserializeObject<Steam>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
                        if (_instance == null)
                        {
                            _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedLoadSettings"]);
                            if (File.Exists(SettingsFile))
                                Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
                            _instance = new Steam { _currentlyModifying = true };
                        }
                        _instance._lastHash = Globals.GetFileMd5(SettingsFile);
                        if (_instance._folderPath.EndsWith(".exe"))
                            _instance._folderPath = Path.GetDirectoryName(_instance._folderPath) ?? string.Join("\\", _instance._folderPath.Split("\\")[..^1]);
                    }
                    else
                    {
                        SaveSettings();
                    }
                    LoadBasicCompat(); // Add missing features in templated platforms system.
                    // Forces lazy values to be instantiated
                    _ = InstalledGames.Value;
                    _ = AppIds.Value;
                    BuildContextMenu();
                    AppData.InitializedClasses.Steam = true;

                    _instance._currentlyModifying = false;

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

        private string _lastHash = "";
        private bool _currentlyModifying;
        public static void SaveSettings() => GeneralFuncs.SaveSettings(SettingsFile, Instance);

        #region Basic Compatability
        public static string GetShortcutImagePath(string gameShortcutName) =>
            Path.Join(GetShortcutImageFolder, PlatformFuncs.RemoveShortcutExt(gameShortcutName) + ".png");
        public static Dictionary<int, string> Shortcuts { get => Instance._shortcuts; set => Instance._shortcuts = value; }
        public static bool ShowShortNotes { get => Instance._showShortNotes; set => Instance._showShortNotes = value; }
        public static Dictionary<string, string> AccountNotes { get => Instance._accountNotes; set => Instance._accountNotes = value; }
        public static string ClosingMethod { get => Instance._closingMethod; set => Instance._closingMethod = value; }
        public static string StartingMethod { get => Instance._startingMethod; set => Instance._startingMethod = value; }
        private static string GetShortcutImageFolder => "img\\shortcuts\\Steam\\";
        private static string GetShortcutImagePath() => Path.Join(Globals.UserDataFolder, "wwwroot\\", GetShortcutImageFolder);
        public static string ShortcutFolder => "LoginCache\\Steam\\Shortcuts\\";
        private static readonly List<string> ShortcutFolders = new () { "%StartMenuAppData%\\Steam\\" };
        private static string GetShortcutIgnoredPath(string shortcut) => Path.Join(ShortcutFolder, shortcut.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));
        private static void LoadBasicCompat()
        {
            if (OperatingSystem.IsWindows())
            {
                // Add image for main platform button:
                var imagePath = Path.Join(GetShortcutImagePath(), "Steam.png");

                if (!File.Exists(imagePath))
                {
                    // Search start menu for Steam.
                    var startMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuAppData%", true), "Steam.lnk", SearchOption.AllDirectories);
                    var commonStartMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuProgramData%", true), "Steam.lnk", SearchOption.AllDirectories);
                    if (startMenuFiles.Length > 0)
                        Globals.SaveIconFromFile(startMenuFiles[0], imagePath);
                    else if (commonStartMenuFiles.Length > 0)
                        Globals.SaveIconFromFile(commonStartMenuFiles[0], imagePath);
                    else
                        Globals.SaveIconFromFile(Exe(), imagePath);
                }


                var cacheShortcuts = ShortcutFolder; // Shortcut cache
                foreach (var sFolder in ShortcutFolders)
                {
                    if (sFolder == "") continue;
                    // Foreach file in folder
                    var desktopShortcutFolder = BasicSwitcherFuncs.ExpandEnvironmentVariables(sFolder, true);
                    if (!Directory.Exists(desktopShortcutFolder)) continue;
                    foreach (var shortcut in new DirectoryInfo(desktopShortcutFolder).GetFiles())
                    {
                        var fName = shortcut.Name;
                        if (PlatformFuncs.RemoveShortcutExt(fName) == "Steam") continue; // Ignore self

                        // Check if in saved shortcuts and If ignored
                        if (File.Exists(GetShortcutIgnoredPath(fName)))
                        {
                            imagePath = Path.Join(GetShortcutImagePath(), PlatformFuncs.RemoveShortcutExt(fName) + ".png");
                            if (File.Exists(imagePath)) File.Delete(imagePath);
                            fName = fName.Replace("_ignored", "");
                            if (Shortcuts.ContainsValue(fName))
                                Shortcuts.Remove(Shortcuts.First(e => e.Value == fName).Key);
                            continue;
                        }

                        Directory.CreateDirectory(cacheShortcuts);
                        var outputShortcut = Path.Join(cacheShortcuts, fName);

                        // Exists and is not ignored: Update shortcut
                        Globals.CopyFile(shortcut.FullName, outputShortcut);
                        // Organization will be saved in HTML/JS
                    }
                }

                // Now get images for all the shortcuts in the folder, as long as they don't already exist:
                List<string> existingShortcuts = new();
                if (Directory.Exists(cacheShortcuts))
                    foreach (var f in new DirectoryInfo(cacheShortcuts).GetFiles())
                    {
                        if (f.Name.Contains("_ignored")) continue;
                        var imageName = PlatformFuncs.RemoveShortcutExt(f.Name) + ".png";
                        imagePath = Path.Join(GetShortcutImagePath(), imageName);
                        existingShortcuts.Add(f.Name);
                        if (!Shortcuts.ContainsValue(f.Name))
                        {
                            // Not found in list, so add!
                            var last = 0;
                            foreach (var (k,_) in Shortcuts)
                                if (k > last) last = k;
                            last += 1;
                            Shortcuts.Add(last, f.Name); // Organization added later
                        }

                        // Extract image and place in wwwroot (Only if not already there):
                        if (!File.Exists(imagePath))
                        {
                            Globals.SaveIconFromFile(f.FullName, imagePath);
                        }
                    }

                foreach (var (i, s) in Shortcuts)
                {
                    if (!existingShortcuts.Contains(s))
                        Shortcuts.Remove(i);
                }
            }

            AppStats.SetGameShortcutCount("Steam", Shortcuts);
            SaveSettings();
        }

        [JSInvokable]
        public static void SaveShortcutOrderSteam(Dictionary<int, string> o)
        {
            Shortcuts = o;
            SaveSettings();
        }

        public static void SetClosingMethod(string method)
        {
            ClosingMethod = method;
            SaveSettings();
        }
        public static void SetStartingMethod(string method)
        {
            StartingMethod = method;
            SaveSettings();
        }

        [JSInvokable]
        public static void HandleShortcutActionSteam(string shortcut, string action)
        {
            if (shortcut == "btnStartPlat") // Start platform requested
            {
                Basic.RunPlatform(Exe(), action == "admin", "", "Steam", StartingMethod);
                return;
            }

            if (!Shortcuts.ContainsValue(shortcut)) return;

            switch (action)
            {
                case "hide":
                {
                    // Remove shortcut from folder, and list.
                    Shortcuts.Remove(Shortcuts.First(e => e.Value == shortcut).Key);
                    var f = Path.Join(ShortcutFolder, shortcut);
                    if (File.Exists(f)) File.Move(f, f.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));

                    // Save.
                    SaveSettings();
                    break;
                }
                case "admin":
                    Basic.RunShortcut(shortcut, "LoginCache\\Steam\\Shortcuts", true, "Steam");
                    break;
            }
        }

        public static readonly Lazy<List<string>> InstalledGames =
            new (SteamSwitcherFuncs.LoadInstalledGames);

        public static readonly Lazy<Dictionary<string, string>> AppIds =
            new (SteamSwitcherFuncs.LoadAppNames);

        #endregion

        // Variables
        [JsonProperty("ForgetAccountEnabled", Order = 0)] private bool _forgetAccountEnabled;
        [JsonProperty("FolderPath", Order = 1)] private string _folderPath = "C:\\Program Files (x86)\\Steam\\";
        [JsonProperty("Steam_Admin", Order = 3)] private bool _admin;
        [JsonProperty("Steam_ShowSteamID", Order = 4)] private bool _showSteamId;
        [JsonProperty("Steam_ShowVAC", Order = 5)] private bool _showVac = true;
        [JsonProperty("Steam_ShowLimited", Order = 6)] private bool _showLimited = true;
        [JsonProperty("Steam_ShowLastLogin", Order = 7)] private bool _showLastLogin = true;
        [JsonProperty("Steam_ShowAccUsername", Order = 8)] private bool _showAccUsername = true;
        [JsonProperty("Steam_TrayAccountName", Order = 9)] private bool _trayAccName;
        [JsonProperty("Steam_ImageExpiryTime", Order = 10)] private int _imageExpiryTime = 7;
        [JsonProperty("Steam_TrayAccNumber", Order = 11)] private int _trayAccNumber = 3;
        [JsonProperty("Steam_OverrideState", Order = 12)] private int _overrideState = -1;
        [JsonProperty("ShortcutsJson", Order = 13)] private Dictionary<int, string> _shortcuts = new();
        [JsonProperty("ClosingMethod", Order = 14)] private string _closingMethod = "TaskKill";
        [JsonProperty("StartingMethod", Order = 15)] private string _startingMethod = "Default";
        [JsonProperty("AutoStart", Order = 16)] private bool _autoStart = true;
        [JsonProperty("ShowShortNotes", Order = 17)] private bool _showShortNotes = true;
        [JsonProperty("AccountNotes", Order = 18)] private Dictionary<string, string> _accountNotes = new();
        [JsonProperty("SteamWebApiKey", Order = 19)] private string _steamWebApiKey = "";
        [JsonProperty("StartSilent", Order = 20)] private bool _startSilent;
        [JsonProperty("OldUi", Order = 21)] private bool _oldUi;
        [JsonProperty("ShowSteamSwitcher", Order = 22)] private bool _showSteamSwitcher = false;
        [JsonProperty("CollectInfo", Order = 23)] private bool _collectInfo = true;
        [JsonIgnore] private bool _desktopShortcut;
        [JsonIgnore] private int _lastAccTimestamp = 0;
        [JsonIgnore] private string _lastAccName = "";
        [JsonIgnore] private bool _steamWebApiWasReset;
        [JsonIgnore] public static readonly string SteamAppsListPath = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsFullListCache.json");
        [JsonIgnore] public static readonly string SteamAppsUserCache = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsUser.json");
        [JsonIgnore] private ObservableCollection<Account> _accounts = new();

        public static int LastAccTimestamp { get => Instance._lastAccTimestamp; set => Instance._lastAccTimestamp = value; }
        public static string LastAccName { get => Instance._lastAccName; set => Instance._lastAccName = value; }

        public static bool ForgetAccountEnabled { get => Instance._forgetAccountEnabled; set => Instance._forgetAccountEnabled = value; }

        public static string FolderPath { get => Instance._folderPath; set => Instance._folderPath = value; }

        public static bool Admin { get => Instance._admin; set => Instance._admin = value; }
        public static bool ShowSteamSwitcher { get => Instance._showSteamSwitcher; set => Instance._showSteamSwitcher = value; }
        public static bool CollectInfo { get => Instance._collectInfo; set => Instance._collectInfo = value; }

        public static bool AutoStart { get => Instance._autoStart; set => Instance._autoStart = value; }

        public static bool ShowSteamId { get => Instance._showSteamId; set => Instance._showSteamId = value; }

        public static bool ShowVac { get => Instance._showVac; set => Instance._showVac = value; }

        public static bool ShowLimited { get => Instance._showLimited; set => Instance._showLimited = value; }
        public static bool ShowLastLogin { get => Instance._showLastLogin; set => Instance._showLastLogin = value; }

        public static bool ShowAccUsername { get => Instance._showAccUsername; set => Instance._showAccUsername = value; }

        public static bool TrayAccName { get => Instance._trayAccName; set => Instance._trayAccName = value; }

        public static int ImageExpiryTime { get => Instance._imageExpiryTime; set => Instance._imageExpiryTime = value; }

        public static int TrayAccNumber { get => Instance._trayAccNumber; set => Instance._trayAccNumber = value; }

        public static int OverrideState { get => Instance._overrideState; set => Instance._overrideState = value; }

        public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }
        public static string SteamWebApiKey { get => Instance._steamWebApiKey; set => Instance._steamWebApiKey = value; }
        public static bool StartSilent { get => Instance._startSilent; set => Instance._startSilent = value; }
        public static bool OldUi { get => Instance._oldUi; set => Instance._oldUi = value; }
        public static bool SteamWebApiWasReset { get => Instance._steamWebApiWasReset; set => Instance._steamWebApiWasReset = value; }
        public static ObservableCollection<Account> Accounts { get => Instance._accounts; set => Instance._accounts = value; }

        // Constants
        public static readonly List<string> Processes = new() { "steam.exe", "SERVICE:steamservice.exe", "steamwebhelper.exe", "GameOverlayUI.exe" };
        public static readonly string VacCacheFile = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\VACCache\\SteamVACCache.json");
        public static readonly string SettingsFile = "SteamSettings.json";
        public static readonly string SteamImagePath = "wwwroot/img/profiles/steam/";
        public static readonly string SteamImagePathHtml = "img/profiles/steam/";
        public static readonly ObservableCollection<MenuItem> Menu = new();
        public static void BuildContextMenu()
        {
            Menu.Clear();
            var menuBuilder = new MenuBuilder(new Tuple<string, object>[]
            {
                new ("Context_SwapTo", "swapTo(-1, event)"),
                new ("Context_LoginAsSubmenu", new Tuple<string, object>[]
                    {
                        new ("Invisible", "swapTo(7, event)"),
                        new ("Offline", "swapTo(0, event)"),
                        new ("Online", "swapTo(1, event)"),
                        new ("Busy", "swapTo(2, event)"),
                        new ("Away", "swapTo(3, event)"),
                        new ("Snooze", "swapTo(4, event)"),
                        new ("LookingToTrade", "swapTo(5, event)"),
                        new ("LookingToPlay", "swapTo(6, event)"),
                    }
                ),
                new ("Context_CopySubmenu", new Tuple<string, object>[]
                {
                    new ("Context_CopyProfileSubmenu", new Tuple<string, object>[]
                    {
                        new ("Context_CommunityUrl", "copy('URL', event)"),
                        new ("Context_CommunityUsername", "copy('Line2', event)"),
                        new ("Context_LoginUsername", "copy('Username', event)"),
                    }),
                    new ("Context_CopySteamIdSubmenu", new Tuple<string, object>[]
                    {
                        new ("Context_Steam_Id", "copy('SteamId', event)"),
                        new ("Context_Steam_Id3", "copy('SteamId3', event)"),
                        new ("Context_Steam_Id32", "copy('SteamId32', event)"),
                        new ("Context_Steam_Id64", "copy('id', event)"),
                    }),
                    new ("Context_CopyOtherSubmenu", new Tuple<string, object>[]
                    {
                        new ("SteamRep", "copy('SteamRep', event)"),
                        new ("SteamID.uk", "copy('SteamID.uk', event)"),
                        new ("SteamID.io", "copy('SteamID.io', event)"),
                        new ("SteamIDFinder.com", "copy('SteamIDFinder.com', event)"),
                    }),
                }),
                new ("Context_CreateShortcut", new Tuple<string, object>[]
                {
                    new ("OnlineDefault", "createShortcut()"),
                    new ("Invisible", "createShortcut(':7')"),
                    new ("Offline", "createShortcut(':0')"),
                    new ("Busy", "createShortcut(':2')"),
                    new ("Away", "createShortcut(':3')"),
                    new ("Snooze", "createShortcut(':4')"),
                    new ("LookingToTrade", "createShortcut(':5')"),
                    new ("LookingToPlay", "createShortcut(':6')"),
                }),
                new ("Forget", "forget(event)"),
                new ("Notes", "showNotes(event)"),
            });

            /* Games submenu, or Game data item */
            MenuItem gameData = null;
            if (File.Exists(SteamAppsUserCache) && AppIds.Value.Count > 0)
            {
                gameData = new MenuItem()
                {
                    Text = "Context_GameDataSubmenu",
                    Children = (
                        from gameId in InstalledGames.Value
                        select new MenuItem()
                        {
                            Text = AppIds.Value.ContainsKey(gameId) ? AppIds.Value[gameId] : gameId,
                            Children = new List<MenuItem>()
                            {
                                new()
                                {
                                    Text = "Context_Game_CopySettingsFrom",
                                    Content = $"CopySettingsFrom(event, '{gameId}')"
                                },
                                new()
                                {
                                    Text = "Context_Game_RestoreSettingsTo",
                                    Content = $"RestoreSettingsTo(event, '{gameId}"
                                },
                                new()
                                {
                                    Text = "Context_Game_BackupData",
                                    Content = $"BackupGameData(event, '{gameId}')"
                                },
                            }
                        }
                    ).ToList()
                };
            }
            menuBuilder.AddItem(new Tuple<string, object>("Context_ManageSubmenu", new Tuple<string, object>[]
                {
                    gameData is not null ?
                        new Tuple<string, object>("Context_GameDataSubmenu", gameData) : null,
                    BasicStats.PlatformHasAnyGames("Steam") ?
                        new Tuple<string, object>("Context_ManageGameStats", "ShowGameStatsSetup(event)") : null,
                    new ("Context_ChangeImage", "changeImage(event)"),
                    new ("Context_Steam_OpenUserdata", "openUserdata(event)")
                }));
            Menu.AddRange(menuBuilder.Result());
        }

        public static string StateToString(int state)
        {
            return state switch
            {
                -1 => Lang["NoDefault"],
                0 => Lang["Offline"],
                1 => Lang["Online"],
                2 => Lang["Busy"],
                3 => Lang["Away"],
                4 => Lang["Snooze"],
                5 => Lang["LookingToTrade"],
                6 => Lang["LookingToPlay"],
                7 => Lang["Invisible"],
                _ => ""
            };
        }

        /// <summary>
        /// Default settings for SteamSettings.json
        /// </summary>
        public static void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.ResetSettings]");
            ForgetAccountEnabled = false;
            FolderPath = "C:\\Program Files (x86)\\Steam\\";
            Admin = false;
            ShowSteamId = false;
            ShowVac = true;
            ShowLimited = true;
            TrayAccName = false;
            ImageExpiryTime = 7;
            TrayAccNumber = 3;
            DesktopShortcut = Shortcut.CheckShortcuts("Steam");
            _ = Shortcut.StartWithWindows_Enabled();

            SaveSettings();
        }

        /// <summary>
        /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
        /// </summary>
        /// <returns>(Steam's path)\config\loginusers.vdf</returns>
        public static string LoginUsersVdf()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.LoginUsersVdf]");
            var path = Path.Join(FolderPath, "config\\loginusers.vdf");
            if (File.Exists(path)) return path;

            FolderPath = "";
            SaveSettings();
            return "RESET_PATH";
        }

        /// <summary>
        /// Get Steam.exe path from SteamSettings.json
        /// </summary>
        /// <returns>Steam.exe's path string</returns>
        public static string Exe() => Path.Join(FolderPath, "Steam.exe");

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public static void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public static string GetSteamIdCssBlock() => ".steamId { display: " + (ShowSteamId ? "block" : "none") + " } .lastLogin { display: " + (ShowLastLogin ? "block" : "none") + " }";
    }
}
