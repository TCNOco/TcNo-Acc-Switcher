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
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Runtime.Versioning;
using System.Text;
using System.Threading.Tasks;
using System.Xml;
using Gameloop.Vdf;
using Gameloop.Vdf.JsonConverter;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Data.Classes;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Steam : ISteam
    {
        private readonly ILang _lang;
        private readonly IAppData _appData;
        private readonly IGeneralFuncs _generalFuncs;
        private readonly IAppStats _appStats;
        private readonly IAppFuncs _appFuncs;
        private readonly IAccountFuncs _accountFuncs;
        private readonly IBasicStats _basicStats;
        private readonly IModalData _modalData;
        private readonly IAppSettings _appSettings;
        private readonly ICurrentPlatform _currentPlatform;
        private readonly Lazy<IBasic> _lBasic;
        private IBasic Basic => _lBasic.Value;
        public Steam(){}
        public Steam(ILang lang, IAppData appData, Lazy<IBasic> basic, IGeneralFuncs generalFuncs, ICurrentPlatform currentPlatform, IAppStats appStats, IAppFuncs appFuncs, IAccountFuncs accountFuncs, IBasicStats basicStats, IModalData modalData, IAppSettings appSettings)
        {
            _lang = lang;
            _appData = appData;
            _lBasic = basic;
            _generalFuncs = generalFuncs;
            _currentPlatform = currentPlatform;
            _appStats = appStats;
            _appFuncs = appFuncs;
            _accountFuncs = accountFuncs;
            _basicStats = basicStats;
            _modalData = modalData;
            _appSettings = appSettings;

            Init();

        }

        private bool _isInit;
        public void Init()
        {
            if (_isInit) return;
            _isInit = true;
            try
            {
                if (File.Exists(_currentPlatform.SettingsFile)) JsonConvert.PopulateObject(File.ReadAllText(_currentPlatform.SettingsFile), this);
                //_instance = JsonConvert.DeserializeObject<AppSettings>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to load BasicSettings", e);
                _ = _generalFuncs.ShowToast("error", _lang["Toast_FailedLoadSettings"]);
                if (File.Exists(SettingsFile))
                    Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
            }

            InstalledGames = LoadInstalledGames();
            AppIds = LoadAppNames();

            LoadBasicCompat(); // Add missing features in templated platforms system.

            //// Forces lazy values to be instantiated
            //_ = InstalledGames.Value;
            //_ = AppIds.Value;

            BuildContextMenu();
            _appData.InitializedClasses.Steam = true;
        }

        public void SaveSettings()
        {
            // Accounts seem to reset when saving, for some reason...
            var accList = _appData.SteamAccounts;
            Data.GeneralFuncs.SaveSettings(SettingsFile, this);
            _appData.SteamAccounts = accList;
        }

        #region Basic Compatability
        public string GetShortcutImagePath(string gameShortcutName) =>
            Path.Join(GetShortcutImageFolder, PlatformFuncs.RemoveShortcutExt(gameShortcutName) + ".png");
        private static string GetShortcutImageFolder => "img\\shortcuts\\Steam\\";
        private static string GetShortcutImagePath() => Path.Join(Globals.UserDataFolder, "wwwroot\\", GetShortcutImageFolder);
        public string ShortcutFolder => "LoginCache\\Steam\\Shortcuts\\";
        private readonly List<string> _shortcutFolders = new () { "%StartMenuAppData%\\Steam\\" };
        private string GetShortcutIgnoredPath(string shortcut) => Path.Join(ShortcutFolder, shortcut.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));
        private void LoadBasicCompat()
        {
            if (OperatingSystem.IsWindows())
            {
                // Add image for main platform button:
                var imagePath = Path.Join(GetShortcutImagePath(), "Steam.png");

                if (!File.Exists(imagePath))
                {
                    // Search start menu for Steam.
                    var startMenuFiles = Directory.GetFiles(BasicFuncs.ExpandEnvironmentVariables("%StartMenuAppData%", true), "Steam.lnk", SearchOption.AllDirectories);
                    var commonStartMenuFiles = Directory.GetFiles(BasicFuncs.ExpandEnvironmentVariables("%StartMenuProgramData%", true), "Steam.lnk", SearchOption.AllDirectories);
                    if (startMenuFiles.Length > 0)
                        Globals.SaveIconFromFile(startMenuFiles[0], imagePath);
                    else if (commonStartMenuFiles.Length > 0)
                        Globals.SaveIconFromFile(commonStartMenuFiles[0], imagePath);
                    else
                        Globals.SaveIconFromFile(Exe(), imagePath);
                }


                var cacheShortcuts = ShortcutFolder; // Shortcut cache
                foreach (var sFolder in _shortcutFolders)
                {
                    if (sFolder == "") continue;
                    // Foreach file in folder
                    var desktopShortcutFolder = BasicSettings.ExpandEnvironmentVariables(sFolder, true);
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

            _appStats.SetGameShortcutCount("Steam", Shortcuts);
            SaveSettings();
        }

        [JSInvokable]
        public void SaveShortcutOrderSteam(Dictionary<int, string> o)
        {
            Shortcuts = o;
            SaveSettings();
        }

        public void SetClosingMethod(string method)
        {
            ClosingMethod = method;
            SaveSettings();
        }
        public void SetStartingMethod(string method)
        {
            StartingMethod = method;
            SaveSettings();
        }

        [JSInvokable]
        public async Task HandleShortcutActionSteam(string shortcut, string action)
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
                    await Basic.RunShortcut(shortcut, "LoginCache\\Steam\\Shortcuts", true, "Steam");
                    break;
            }
        }

        public List<string> InstalledGames { get; set; }
        public Dictionary<string, string> AppIds { get; set; }

        #endregion



        [JsonProperty(Order = 0)] public bool ForgetAccountEnabled { get;set; }
        [JsonProperty(Order = 1)] public string FolderPath { get; set; } = "C:\\Program Files (x86)\\Steam\\";
        [JsonProperty(Order = 3)] public bool Admin { get; set; }
        [JsonProperty(Order = 4)] public bool ShowSteamId { get; set; }
        [JsonProperty(Order = 5)] public bool ShowVac { get; set; } = true;
        [JsonProperty(Order = 6)] public bool ShowLimited { get; set; } = true;
        [JsonProperty(Order = 7)] public bool ShowLastLogin { get; set; } = true;
        [JsonProperty(Order = 8)] public bool ShowAccUsername { get; set; } = true;
        [JsonProperty(Order = 9)] public bool TrayAccName { get; set; }
        [JsonProperty(Order = 10)] public int ImageExpiryTime { get; set; } = 7;
        [JsonProperty(Order = 11)] public int TrayAccNumber { get; set; } = 3;
        [JsonProperty(Order = 12)] public int OverrideState { get; set; } = -1;
        [JsonProperty(Order = 13)] public Dictionary<int, string> Shortcuts { get; set; } = new();
        [JsonProperty(Order = 14)] public string ClosingMethod { get; set; } = "TaskKill";
        [JsonProperty(Order = 15)] public string StartingMethod { get; set; } = "Default";
        [JsonProperty(Order = 16)] public bool AutoStart { get; set; } = true;
        [JsonProperty(Order = 17)] public bool ShowShortNotes { get; set; } = true;
        [JsonProperty(Order = 19)] public string SteamWebApiKey { get; set; } = "";
        [JsonProperty(Order = 20)] public bool StartSilent { get; set; }
        [JsonProperty(Order = 21)] public Dictionary<string, string> CustomAccNames { get; set; }
        [JsonIgnore] public bool DesktopShortcut { get; set; }
        [JsonIgnore] public int LastAccTimestamp { get; set; }
        [JsonIgnore] public string LastAccSteamId { get; set; } = "";
        [JsonIgnore] public bool SteamWebApiWasReset { get; set; }
        [JsonIgnore] public string SteamAppsListPath { get; set; } = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsFullListCache.json");
        [JsonIgnore] public string SteamAppsUserCache { get; set; } = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsUser.json");

        // Constants
        public List<string> Processes { get; set; } = new() { "steam.exe", "SERVICE:steamservice.exe", "steamwebhelper.exe", "GameOverlayUI.exe" };
        public string VacCacheFile { get; set; } = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\VACCache\\SteamVACCache.json");
        public string SettingsFile { get; set; } = "SteamSettings.json";
        public string SteamImagePath { get; set; } = "wwwroot/img/profiles/steam/";
        public string SteamImagePathHtml { get; set; } = "img/profiles/steam/";
        public ObservableCollection<MenuItem> Menu { get; set; } = new();
        public void BuildContextMenu()
        {
            Menu.Clear();

            /* Games submenu, or Game data item */
            MenuItem gameData = null;
            if (File.Exists(SteamAppsUserCache) && AppIds.Count > 0)
            {
                var menuItems = new List<MenuItem>();
                foreach (var gameId in InstalledGames)
                {
                    menuItems.Add(new MenuItem
                    {
                        Text = AppIds.ContainsKey(gameId) ? AppIds[gameId] : gameId,
                        Children = new List<MenuItem>{
                            new() {
                                Text = _lang["Context_Game_CopySettingsFrom"],
                                MenuAction = async () => await CopySettingsFrom(gameId)
                            },
                            new() {
                                Text = _lang["Context_Game_RestoreSettingsTo"],
                                MenuAction = async () => await RestoreSettingsTo(gameId)
                            },
                            new() {
                                Text = _lang["Context_Game_BackupData"],
                                MenuAction = async () => await BackupGameData(gameId)
                            }
                        }
                    });
                }

                gameData = new MenuItem()
                {
                    Text = "Context_GameDataSubmenu",
                    Children = menuItems
                };
            }

            // Prepare menu
            var menuBuilder = new MenuBuilder(new Tuple<string, object>[]
            {
                new ("Context_SwapTo", new Action(async () => await _appFuncs.SwapToAccount())),
                new ("Context_LoginAsSubmenu", new Tuple<string, object>[]
                    {
                        new ("Invisible", new Action(async () => await _appFuncs.SwapToAccount(7))),
                        new ("Offline", new Action(async () => await _appFuncs.SwapToAccount(0))),
                        new ("Online", new Action(async () => await _appFuncs.SwapToAccount(1))),
                        new ("Busy", new Action(async () => await _appFuncs.SwapToAccount(2))),
                        new ("Away", new Action(async () => await _appFuncs.SwapToAccount(3))),
                        new ("Snooze", new Action(async () => await _appFuncs.SwapToAccount(4))),
                        new ("LookingToTrade", new Action(async () => await _appFuncs.SwapToAccount(5))),
                        new ("LookingToPlay", new Action(async () => await _appFuncs.SwapToAccount(6))),
                    }
                ),
                new ("Context_CopySubmenu", new Tuple<string, object>[]
                {
                    new ("Context_CopyProfileSubmenu", new Tuple<string, object>[]
                    {
                        new ("Context_CommunityUrl", new Action(async () => await _appFuncs.CopyText($"https://steamcommunity.com/profiles/{_appData.SelectedAccountId}"))),
                        new ("Context_CommunityUsername", new Action(async () => await _appFuncs.CopyText(_appData.SelectedAccount.DisplayName))),
                        new ("Context_LoginUsername", new Action(async () => await _appFuncs.CopyText(_appData.SelectedAccount.LoginUsername))),
                    }),
                    new ("Context_CopySteamIdSubmenu", new Tuple<string, object>[]
                    {
                        new ("Context_Steam_Id", new Action(async () => await _appFuncs.CopyText(new SteamIdConvert(_appData.SelectedAccountId).Id))),
                        new ("Context_Steam_Id3", new Action(async () => await _appFuncs.CopyText(new SteamIdConvert(_appData.SelectedAccountId).Id3))),
                        new ("Context_Steam_Id32", new Action(async () => await _appFuncs.CopyText(new SteamIdConvert(_appData.SelectedAccountId).Id32))),
                        new ("Context_Steam_Id64", new Action(async () => await _appFuncs.CopyText(new SteamIdConvert(_appData.SelectedAccountId).Id64))),
                    }),
                    new ("Context_CopyOtherSubmenu", new Tuple<string, object>[]
                    {
                        new ("SteamRep", new Action(async () => await _appFuncs.CopyText($"https://steamrep.com/search?q={_appData.SelectedAccountId}"))),
                        new ("SteamID.uk", new Action(async () => await _appFuncs.CopyText($"https://steamid.uk/profile/{_appData.SelectedAccountId}"))),
                        new ("SteamID.io", new Action(async () => await _appFuncs.CopyText($"https://steamid.io/lookup/{_appData.SelectedAccountId}"))),
                        new ("SteamIDFinder.com", new Action(async () => await _appFuncs.CopyText($"https://steamidfinder.com/lookup/{_appData.SelectedAccountId}"))),
                    }),
                }),
                new ("Context_CreateShortcut", new Tuple<string, object>[]
                {
                    new ("OnlineDefault", new Action(async () => await _generalFuncs.CreateShortcut())),
                    new ("Invisible", new Action(async () => await _generalFuncs.CreateShortcut(":7"))),
                    new ("Offline", new Action(async () => await _generalFuncs.CreateShortcut(":0"))),
                    new ("Busy", new Action(async () => await _generalFuncs.CreateShortcut(":2"))),
                    new ("Away", new Action(async () => await _generalFuncs.CreateShortcut(":3"))),
                    new ("Snooze", new Action(async () => await _generalFuncs.CreateShortcut(":4"))),
                    new ("LookingToTrade", new Action(async () => await _generalFuncs.CreateShortcut(":5"))),
                    new ("LookingToPlay", new Action(async () => await _generalFuncs.CreateShortcut(":6"))),
                }),
                new ("Forget", new Action(async () => await _appFuncs.ForgetAccount())),
                new ("Notes", new Action(() => _modalData.ShowModal("notes"))),
                new ("Context_ManageSubmenu", new[]
                {
                    gameData is not null
                        ? new Tuple<string, object>("Context_GameDataSubmenu", gameData)
                        : null, _basicStats.PlatformHasAnyGames("Steam")
                            ? new Tuple<string, object>("Context_ManageGameStats", "ShowGameStatsSetup(event)")
                            : null,
                    new ("Context_ChangeImage", new Action(_modalData.ShowChangeAccImageModal)),
                    new ("Context_Steam_OpenUserdata", new Action(SteamOpenUserdata)),
                    new ("Context_ChangeName", new Action(_modalData.ShowChangeUsernameModal))
                })
            });

            Menu.AddRange(menuBuilder.Result());
        }

        public void SteamOpenUserdata()
        {
            var steamId32 = new SteamIdConvert(_appData.SelectedAccountId);
            var folder = Path.Join(FolderPath, $"userdata\\{steamId32.Id32}");
            if (Directory.Exists(folder)) _ = Process.Start("explorer.exe", folder);
            else _ = _generalFuncs.ShowToast("error", _lang["Toast_NoFindSteamUserdata"], _lang["Failed"], "toastarea");
        }

        public string StateToString(int state)
        {
            return state switch
            {
                -1 => _lang["NoDefault"],
                0 => _lang["Offline"],
                1 => _lang["Online"],
                2 => _lang["Busy"],
                3 => _lang["Away"],
                4 => _lang["Snooze"],
                5 => _lang["LookingToTrade"],
                6 => _lang["LookingToPlay"],
                7 => _lang["Invisible"],
                _ => ""
            };
        }

        /// <summary>
        /// Default settings for SteamSettings.json
        /// </summary>
        public void ResetSettings()
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
            DesktopShortcut = ShortcutFuncs.CheckShortcuts(_appSettings, "Steam");
            _ = ShortcutFuncs.StartWithWindows_Enabled();

            SaveSettings();
        }

        /// <summary>
        /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
        /// </summary>
        /// <returns>(Steam's path)\config\loginusers.vdf</returns>
        public string LoginUsersVdf()
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
        public string Exe() => Path.Join(FolderPath, "Steam.exe");

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public string GetSteamIdCssBlock() => ".steamId { display: " + (ShowSteamId ? "block" : "none") + " } .lastLogin { display: " + (ShowLastLogin ? "block" : "none") + " }";



        #region STEAM_SWITCHER_MAIN

        private static string GetName(SteamUserAcc su) => string.IsNullOrWhiteSpace(su.Name) ? su.AccName : su.Name;

        /// <summary>
        /// Main function for Steam Account Switcher. Run on load.
        /// Collects accounts from Steam's loginusers.vdf
        /// Prepares images and VAC/Limited status
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public async Task LoadProfiles()
        {
            if (_appData.SteamLoadingProfiles) return;
            Globals.DebugWriteLine(@"[Func:Steam\Steam.LoadProfiles] Loading Steam profiles");
            _appData.SteamLoadingProfiles = true;

            _appData.SteamUsers = await GetSteamUsers(LoginUsersVdf());

            // Order
            if (File.Exists("LoginCache\\Steam\\order.json"))
            {
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\Steam\\order.json").ConfigureAwait(false));
                if (savedOrder != null)
                {
                    var index = 0;
                    if (savedOrder is { Count: > 0 })
                        foreach (var acc in from i in savedOrder where _appData.SteamUsers.Any(x => x.SteamId == i) select _appData.SteamUsers.Single(x => x.SteamId == i))
                        {
                            _appData.SteamUsers.Remove(acc);
                            _appData.SteamUsers.Insert(Math.Min(index, _appData.SteamUsers.Count), acc);
                            index++;
                        }
                }
            }

            // If Steam Web API key to be used instead
            if (SteamWebApiKey != "")
            {
                // Handle all image downloads
                await WebApiPrepareImages();
                await WebApiPrepareBans();

                // Key was fine? Continue. If not, the non-api method will be used.
                if (!SteamWebApiWasReset)
                {
                    _appData.SteamAccounts.Clear();
                    foreach (var su in _appData.SteamUsers)
                    {
                        InsertAccount(su);
                    }
                }
            }

            // If Steam Web API key was not used, or was reset (encountered an error)
            if (SteamWebApiKey == "" || SteamWebApiWasReset)
            {
                SteamWebApiWasReset = false;

                // Load cached ban info
                LoadCachedBanInfo();
                _appData.SteamAccounts.Clear();

                foreach (var su in _appData.SteamUsers)
                {
                    // If not cached: Get ban status and images
                    // If cached: Just get images
                    PrepareProfile(su.SteamId, !su.BanInfoLoaded); // Get ban status as well if was not loaded (missing from cache)

                    InsertAccount(su);
                }

                SaveVacInfo();
            }

            // Load notes
            _appFuncs.LoadNotes();

            await _generalFuncs.FinaliseAccountList();
            _appStats.SetAccountCount("Steam", _appData.SteamUsers.Count);
            _appData.SteamLoadingProfiles = false;
        }

        private void InsertAccount(SteamUserAcc su)
        {
            var account = new Account
            {
                Platform = "Steam",
                AccountId = su.SteamId,
                DisplayName = _generalFuncs.EscapeText(GetName(su)),
                Classes = new Account.ClassCollection()
                {
                    Image = (ShowVac && su.Vac ? " status_vac" : "") + (ShowLimited && su.Limited ? " status_limited" : ""),
                    Line0 = "streamerCensor",
                    Line2 = "streamerCensor steamId",
                    Line3 = "lastLogin"
                },
                LoginUsername = su.AccName,
                UserStats = _basicStats.GetUserStatsAllGamesMarkup("Steam", su.SteamId),
                ImagePath = su.ImgUrl,
                Line0 = ShowAccUsername ? su.AccName : "",
                Line2 = su.SteamId,
                Line3 = UnixTimeStampToDateTime(su.LastLogin)
            };
            _appData.SteamAccounts.Add(account);
        }


        /// <summary>
        /// This relies on Steam updating loginusers.vdf. It could go out of sync assuming it's not updated reliably. There is likely a better way to do this.
        /// I am avoiding using the Steam API because it's another DLL to include, but is the next best thing - I assume.
        /// </summary>
        public string GetCurrentAccountId(bool getNumericId = false)
        {
            Globals.DebugWriteLine($@"[Func:Steam\Steam.GetCurrentAccountId]");
            try
            {
                // Refreshing the list of SteamUsers doesn't help here when switching, as the account list is not updated by Steam just yet.
                SteamUserAcc mostRecent = null;
                foreach (var su in _appData.SteamUsers)
                {
                    int.TryParse(su.LastLogin, out var last);

                    int.TryParse(mostRecent?.LastLogin, out var recent);

                    if (mostRecent == null || last > recent)
                        mostRecent = su;
                }

                int.TryParse(mostRecent?.LastLogin, out var mrTimestamp);

                if (LastAccTimestamp > mrTimestamp)
                {
                    return LastAccSteamId;
                }

                if (getNumericId) return mostRecent?.SteamId ?? "";
                return mostRecent?.AccName ?? "";
            }
            catch (Exception)
            {
                //
            }

            return "";
        }

        /// <summary>
        /// Takes loginusers.vdf and iterates through each account, loading details into output Steamuser list.
        /// </summary>
        /// <param name="loginUserPath">loginusers.vdf path</param>
        /// <returns>List of SteamUser classes, from loginusers.vdf</returns>
        public async Task<List<SteamUserAcc>> GetSteamUsers(string loginUserPath)
        {
            Globals.DebugWriteLine($@"[Func:Steam\Steam.GetSteamUsers] Getting list of Steam users from {loginUserPath}");
            _ = Directory.CreateDirectory("wwwroot/img/profiles");

            if (LoadFromVdf(loginUserPath, out var userAccounts)) return userAccounts;

            // Didn't work, try last file, if exists.
            var lastVdf = loginUserPath.Replace(".vdf", ".vdf_last");
            if (!File.Exists(lastVdf) || !LoadFromVdf(lastVdf, out userAccounts)) return new List<SteamUserAcc>();

            await _generalFuncs.ShowToast("info", _lang["Toast_Steam_VdfLast"], _lang["Toast_PartiallyFixed"], "toastarea", 10000);
            return userAccounts;
        }

        // TODO: Copy the GitHub repo that downloads the latest apps, and shares as XML and CSV. Then remove those, and replace it with compressing with 7-zip. Download the latest 7-zip archive here, decompress then read. It takes literally ~1.5MB instead of ~8MB. HUGE saving for super slow internet.
        /// <summary>
        /// Fetches the names corresponding to each game ID from Valve's API.
        /// </summary>
        private string FetchSteamAppsData() => File.Exists(SteamAppsListPath) ? File.ReadAllText(SteamAppsListPath) : "";

        public async Task DownloadSteamAppsData()
        {
            await _generalFuncs.ShowToast("info", _lang["Toast_Steam_DownloadingAppIds"], renderTo: "toastarea");

            try
            {
                // Save to file
                var file = new FileInfo(SteamAppsListPath);
                if (file.Exists) file.Delete();
                if (Globals.ReadWebUrl("https://api.steampowered.com/ISteamApps/GetAppList/v2/", out var appList))
                    await File.WriteAllTextAsync(file.FullName, appList);
                else
                    throw new Exception("Failed to download Steam apps list.");
            }
            catch (Exception e)
            {
                Globals.DebugWriteLine($@"Error downloading Steam app list: {e}");
            }

            await _generalFuncs.ShowToast("info", _lang["Toast_Steam_DownloadingAppIdsComplete"], renderTo: "toastarea");
        }

        /// <summary>
        /// Given a JSON string fetched from Valve's API, return a dictionary mapping game IDs to names.
        /// </summary>
        /// <param name="text">A JSON string matching Valve's API format</param>
        /// <returns></returns>
        private static Dictionary<string, string> ParseSteamAppsText(string text)
        {
            if (text == "") return new Dictionary<string, string>();

            var appIds = new Dictionary<string, string>();
            try
            {
                var json = JObject.Parse(text);
                foreach (var app in json["applist"]?["apps"]!)
                {
                    if (appIds.ContainsKey(app["appid"]!.Value<string>()!)) continue;
                    appIds.Add(app["appid"].Value<string>()!, app["name"]!.Value<string>());
                }
            }
            catch (Exception e)
            {
                Globals.DebugWriteLine($@"Error parsing Steam app list: {e}");
            }
            return appIds;
        }

        public Dictionary<string, string> LoadAppNames()
        {
            // Check if cached Steam AppId list is downloaded
            // If not, skip. Download is handled in a background task.
            if (!File.Exists(SteamAppsListPath))
            {
                // Download Steam AppId list if not already.
                Task.Run(DownloadSteamAppsData).ContinueWith(_ =>
                {
                    var names = LoadAppNames();
                    foreach (var kv in names)
                    {
                        try
                        {
                            AppIds.Add(kv.Key, kv.Value);
                        }
                        catch (Exception e)
                        {
                            Console.WriteLine(e.ToString());
                        }
                    }
                    BuildContextMenu();
                });
                return new Dictionary<string, string>();
            }

            var cacheFilePath = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsUser.json");
            var appIds = new Dictionary<string, string>();
            var gameList = InstalledGames;
            try
            {
                // Check if all the IDs we need are in the cache, i.e. the user has not installed any new games.
                if (File.Exists(cacheFilePath))
                {
                    var cachedAppIds = ParseSteamAppsText(File.ReadAllText(cacheFilePath));
                    if (gameList.All(id => cachedAppIds.ContainsKey(id)))
                    {
                        return cachedAppIds;
                    }
                }

                // If the cache is missing or incomplete, fetch app Ids from Steam's API
                appIds =
                    (from game in ParseSteamAppsText(FetchSteamAppsData())
                     where gameList.Contains(game.Key)
                     select game)
                    .ToDictionary(game => game.Key, game => game.Value);

                // Downloading app list for the first time.
                if (appIds.Count == 0) return appIds;

                // Add any missing games as just the appid. These can include games/apps not on steam (developer Steam accounts), or otherwise removed games from
                if (appIds.Count != gameList.Count)
                {
                    foreach (var g in (from game in gameList where !appIds.ContainsKey(game) select game))
                    {
                        appIds.Add(g, g);
                    }
                }


                // Write the IDs of currently installed games to the cache
                dynamic cacheObject = new System.Dynamic.ExpandoObject();
                cacheObject.applist = new System.Dynamic.ExpandoObject();
                cacheObject.applist.apps = (from app in appIds
                                            select new { appid = app.Key, name = app.Value }).ToArray();
                File.WriteAllText(cacheFilePath, JObject.FromObject(cacheObject).ToString(Newtonsoft.Json.Formatting.None));
            }
            catch (Exception e)
            {
                Globals.DebugWriteLine($@"Error Loading names for Steam game IDs: {e}");
            }
            return appIds;
        }
        public static bool BackupGameDataFolder(string folder)
        {
            var backupFolder = folder + "_switcher_backup";
            if (!Directory.Exists(folder) || Directory.Exists(backupFolder)) return false;
            Globals.CopyDirectory(folder, backupFolder, true);
            return true;
        }
        public List<string> LoadInstalledGames()
        {
            List<string> gameIds;
            try
            {
                var libraryFile = Path.Join(FolderPath, "\\steamapps\\libraryfolders.vdf");
                var libraryVdf = VdfConvert.Deserialize(File.ReadAllText(libraryFile));
                var library = new JObject { libraryVdf.ToJson() };
                gameIds = library["libraryfolders"]!
                    .SelectMany(folder => ((JObject)folder.First?["apps"])?.Properties()
                        .Select(p => p.Name))
                    .ToList();
            }
            catch (Exception e)
            {
                Globals.WriteToLog("ERROR: Could not fetch Steam game library.\nDetails: " + e);
                gameIds = new List<string>();
            }
            return gameIds;
        }
        private bool LoadFromVdf(string vdf, out List<SteamUserAcc> userAccounts)
        {
            userAccounts = new List<SteamUserAcc>();

            try
            {
                var vdfText = VerifyVdfText(vdf);

                var loginUsersVToken = VdfConvert.Deserialize(vdfText);
                var loginUsers = new JObject { loginUsersVToken.ToJson() };

                if (loginUsers["users"] != null)
                {
                    foreach (var jUsr in loginUsers["users"])
                    {
                        try
                        {
                            var jOUsr = (JObject)jUsr.First();
                            var steamId = jUsr.ToObject<JProperty>()?.Name;
                            if (string.IsNullOrWhiteSpace(steamId) && string.IsNullOrWhiteSpace(jOUsr.GetValue("AccountName", StringComparison.OrdinalIgnoreCase)?.Value<string>())) continue;

                            userAccounts.Add(new SteamUserAcc
                            {
                                Name = jOUsr.GetValue("PersonaName", StringComparison.OrdinalIgnoreCase)?.Value<string>() ?? "PersonaNotFound",
                                AccName = jOUsr.GetValue("AccountName", StringComparison.OrdinalIgnoreCase)?.Value<string>() ?? "NameNotFound",
                                SteamId = steamId,
                                ImgUrl = "img/QuestionMark.jpg",
                                LastLogin = jOUsr.GetValue("Timestamp", StringComparison.OrdinalIgnoreCase)?.Value<string>() ?? "0",
                                OfflineMode = !string.IsNullOrWhiteSpace(jOUsr.GetValue("WantsOfflineMode", StringComparison.OrdinalIgnoreCase)?.Value<string>())
                                    ? jOUsr.GetValue("WantsOfflineMode", StringComparison.OrdinalIgnoreCase)?.Value<string>() : "0"
                            });
                        }
                        catch (Exception)
                        {
                            Globals.WriteToLog("Could not import Steam user. Please send your loginusers.vdf file to TechNobo for analysis.");
                        }
                    }
                }
            }
            catch (FileNotFoundException)
            {
                _ = _generalFuncs.ShowToast("error", _lang["Toast_Steam_FailedLoginusers"], _lang["NotFound"], "toastarea");
                return false;
            }
            catch (AggregateException)
            {
                _ = _generalFuncs.ShowToast("error", _lang["Toast_Steam_FailedLoginusers"], _lang["Error"], "toastarea");
                return false;
            }
            catch (Exception)
            {
                _ = _generalFuncs.ShowToast("error", _lang["Toast_Steam_FailedLoginusers"], _lang["Error"], "toastarea");
                return false;
            }


            return true;
        }

        public async Task ChangeUsername()
        {
            CustomAccNames[_appData.SelectedAccountId] = _modalData.TextInput.LastString;
            SaveSettings();

            _appData.SelectedAccount.NotifyDataChanged();
            _modalData.TextInputNotifyDataChanged();
            await _generalFuncs.ShowToast("success", _lang["Toast_ChangedUsername"], renderTo: "toastarea");
        }
        private static string VerifyVdfText(string loginUserPath)
        {
            var original = Globals.ReadAllText(loginUserPath);
            var vdf = original;
            // Replaces double quotes, sometimes added by mistake (?) with single, as they should be.
            vdf = vdf.Replace("\"\"", "\"");
            if (original == vdf) return vdf;

            // Save original file if different.
            try
            {
                File.WriteAllText(loginUserPath, vdf);
            }
            catch (Exception)
            {
                //
            }
            return vdf;
        }

        /// <summary>
        /// Loads ban info from cache file
        /// </summary>
        /// <returns>Whether file was loaded. False if deleted ~ failed to load.</returns>
        public void LoadCachedBanInfo()
        {
            Globals.DebugWriteLine(@"[Func:Steam\Steam.LoadCachedBanInfo]");
            _ = _generalFuncs.DeletedOutdatedFile(VacCacheFile, ImageExpiryTime);
            if (!File.Exists(VacCacheFile)) return;

            // Load list of banStatus
            var banInfoList = JsonConvert.DeserializeObject<List<VacStatus>>(Globals.ReadAllText(VacCacheFile));

            if (banInfoList == null) return;
            foreach (var su in _appData.SteamUsers)
            {
                var banInfo = banInfoList.FirstOrDefault(x => x.SteamId == su.SteamId);
                su.BanInfoLoaded = banInfo != null;
                if (!su.BanInfoLoaded) continue;

                su.Vac = banInfo!.Vac;
                su.Limited = banInfo.Ltd;
            }
        }

        /// <summary>
        /// Class for storing SteamID, VAC status and Limited status.
        /// </summary>
        public class VacStatus
        {
            [JsonProperty("SteamID", Order = 0)] public string SteamId { get; set; }
            [JsonProperty("Vac", Order = 1)] public bool Vac { get; set; }
            [JsonProperty("Ltd", Order = 2)] public bool Ltd { get; set; }
        }

        /// <summary>
        /// Saves List of VacStatus into cache file as JSON.
        /// </summary>
        public void SaveVacInfo()
        {
            var vsList = _appData.SteamUsers.Select(su => new VacStatus
            {
                SteamId = su.SteamId,
                Vac = su.Vac,
                Ltd = su.Limited
            })
                .ToList();

            _ = Directory.CreateDirectory(Path.GetDirectoryName(VacCacheFile) ?? string.Empty);
            File.WriteAllText(VacCacheFile, JsonConvert.SerializeObject(vsList));
        }

        /// <summary>
        /// Converts Unix Timestamp string to DateTime
        /// </summary>
        public string UnixTimeStampToDateTime(string stringUnixTimeStamp)
        {
            if (!double.TryParse(stringUnixTimeStamp, out var unixTimeStamp)) return "";
            // Unix timestamp is seconds past epoch
            var dtDateTime = new DateTime(1970, 1, 1, 0, 0, 0, 0, DateTimeKind.Utc);
            dtDateTime = dtDateTime.AddSeconds(unixTimeStamp).ToLocalTime();
            return dtDateTime.ToString(CultureInfo.InvariantCulture);
        }

        /// <summary>
        /// Deletes outdated/invalid profile images (If they exist)
        /// Then downloads a new copy from Steam
        /// </summary>
        /// <param name="steamId"></param>
        /// <param name="noCache">Whether a new copy of XML data should be downloaded</param>
        /// <returns></returns>
        public void PrepareProfile(string steamId, bool noCache)
        {
            var su = _appData.SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
            if (su == null) return;
            Globals.DebugWriteLine(
                $@"[Func:Steam\Steam.PrepareProfile] Preparing image and ban info for: {su.SteamId.Substring(su.SteamId.Length - 4, 4)}");
            _ = Directory.CreateDirectory(SteamImagePath);

            var dlDir = $"{SteamImagePath}{su.SteamId}.jpg";
            var cachedFile = $"LoginCache/Steam/VACCache/{su.SteamId}.xml";

            if (noCache)
            {
                // 1. Clear cached image and profile data
                Globals.DeleteFile(dlDir);
                Globals.DeleteFile(cachedFile);
            }
            else
            {
                // 1.1 Clear old images
                _ = _generalFuncs.DeletedOutdatedFile(dlDir, ImageExpiryTime);
                _ = _generalFuncs.DeletedInvalidImage(dlDir);

                // 1.2 Clear old cached profile data
                _ = _generalFuncs.DeletedOutdatedFile(cachedFile, 1);
            }

            // 2. Download new copy of user data if not cached.
            _ = Directory.CreateDirectory("LoginCache/Steam/VACCache/");
            var profileXml = new XmlDocument();
            try
            {
                profileXml.Load(File.Exists(cachedFile)
                    ? cachedFile
                    : $"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to load Steam account XML. ", e);
                // Issue was caused by web. Throw.
                if (!File.Exists(cachedFile)) throw;
                Globals.DeleteFile(cachedFile);
                Globals.WriteToLog("The issue was the local XML file. It has been deleted and re-downloaded.");
                // Issue was caused by cached fil. Delete, and re-download.
                profileXml.Load($"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
            }

            if (!File.Exists(cachedFile)) profileXml.Save(cachedFile);


            if (profileXml.DocumentElement == null ||
                profileXml.DocumentElement.SelectNodes("/profile/privacyMessage")?.Count != 0) return;


            // 3. Set ban info in SteamUsers
            ProcessSteamUserXml(profileXml);

            // 4. Download profile image if missing
            if (!File.Exists(dlDir))
            {
                try
                {
                    Globals.DownloadFile(su.ImageDownloadUrl, dlDir);
                    su.ImgUrl = $"{SteamImagePathHtml}{su.SteamId}.jpg";
                    return;
                }
                catch (WebException ex)
                {
                    if (ex.HResult == -2146233079) return;
                    Globals.WriteToLog(
                        "ERROR: Could not connect and download Steam profile's image from Steam servers.\nCheck your internet connection.\n\nDetails: " +
                        ex);
                }
            }

            su.ImgUrl = File.Exists(dlDir)
                ? $"{SteamImagePathHtml}{su.SteamId}.jpg"
                : "img/QuestionMark.jpg";
        }

        /// <summary>
        /// Gets ban status and image from input XML Document.
        /// </summary>
        /// <param name="profileXml">User's profile XML string</param>
        private void ProcessSteamUserXml(XmlDocument profileXml)
        {
            Globals.DebugWriteLine(@"[Func:Steam\Steam.XmlGetVacLimitedStatus] Get Ban status for Steam account.");

            // Read SteamID and select the correct Steam User to edit
            var steamId = profileXml.DocumentElement?.SelectNodes("/profile/steamID64")?[0]?.InnerText;
            if (steamId == null) return;
            var su = _appData.SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
            if (su == null) return;

            // Set ban info in SteamUsers
            try
            {
                // Set ban info
                if (profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0] != null)
                    su.Vac = profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0]?.InnerText == "1";
                if (profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0] != null)
                    su.Limited = profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0]?.InnerText == "1";
                if (profileXml.DocumentElement.SelectNodes("/profile/avatarFull")?[0] != null)
                    su.ImageDownloadUrl = profileXml.DocumentElement.SelectNodes("/profile/avatarFull")?[0]?.InnerText;
            }
            catch (NullReferenceException)
            {
                Globals.DebugWriteLine(@"[Func:Steam\Steam.XmlGetVacLimitedStatus] SUPPRESSED ERROR: NullReferenceException");
            }
        }

        /// <summary>
        /// Gets a list of profile image URLs for all profiles
        /// This replaces GetUserImageUrl, and does it for every SteamId at once
        /// </summary>
        /// <returns>Dictionary of [Profile image URL] = Destination file path</returns>
        private async Task<Dictionary<string, string>> WebApiGetImageList(IReadOnlyCollection<SteamUserAcc> steamUsers)
        {
            var images = new Dictionary<string, string>();
            // Web API can take up to 100 items at once.
            for (var i = 0; i < steamUsers.Count; i += 100)
            {
                var steamUsersGroup = steamUsers.Skip(i).Take(100);
                var steamIds = steamUsersGroup.Select(su => su.SteamId).ToList();
                var uri =
                    $"https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key={SteamWebApiKey}&steamids={string.Join(',', steamIds)}";
                Globals.ReadWebUrl(uri, out var jsonString);
                if (!await IsSteamApiKeyValid(jsonString)) return images;
                var json = JObject.Parse(jsonString);

                if (!json.ContainsKey("response")) return images;
                json = json.Value<JObject>("response");
                if (!json!.ContainsKey("players")) return images;
                var players = json.Value<JArray>("players");

                foreach (var player in players!)
                {
                    var steamId = player["steamid"]!.Value<string>();
                    var imageUrl = player["avatarfull"]!.Value<string>();
                    if (steamId != null && imageUrl != null) images.Add(imageUrl, $"{SteamImagePath}/{steamId}.jpg");
                }
            }

            return images;
        }

        /// <summary>
        /// Download all missing or outdated Steam profile images, multi-threaded
        /// </summary>
        private async Task WebApiPrepareImages()
        {
            // Create a queue of SteamUsers to download images for
            var queue = new List<SteamUserAcc>();

            foreach (var su in _appData.SteamUsers)
            {
                var dlDir = $"{SteamImagePath}{su.SteamId}.jpg";
                // Delete outdated file, if it exists
                var wasDeleted = _generalFuncs.DeletedOutdatedFile(dlDir, ImageExpiryTime);
                // ... & invalid files
                wasDeleted = wasDeleted || _generalFuncs.DeletedInvalidImage(dlDir);

                if (wasDeleted) queue.Add(su);
            }

            // Either return if queue is empty, or download queued images and then continue.
            if (queue.Count != 0)
            {
                await _generalFuncs.ShowToast("info", _lang["Toast_DownloadingProfileData"], renderTo: "toastarea");
                await Globals.MultiThreadParallelDownloads(await WebApiGetImageList(queue)).ConfigureAwait(true);
            }

            // Set the correct path
            foreach (var su in _appData.SteamUsers)
            {
                su.ImgUrl = $"{SteamImagePathHtml}{su.SteamId}.jpg";
            }
        }

        /// <summary>
        /// Checks ban status for all AppData.SteamUsers
        /// Then updates with ban info
        /// Saves VAC info into SteamVACCache.json as well for caching
        /// </summary>
        private async Task WebApiGetBans()
        {
            // Web API can take up to 100 items at once.
            for (var i = 0; i < _appData.SteamUsers.Count; i += 100)
            {
                var steamUsersGroup = _appData.SteamUsers.Skip(i).Take(100);
                var steamIds = steamUsersGroup.Select(su => su.SteamId).ToList();
                var uri =
                    $"https://api.steampowered.com/ISteamUser/GetPlayerBans/v0001/?key={SteamWebApiKey}&steamids={string.Join(',', steamIds)}";
                try
                {
                    Globals.ReadWebUrl(uri, out var jsonString);
                    if (!await IsSteamApiKeyValid(jsonString)) return;
                    var json = JObject.Parse(jsonString);

                    if (!json!.ContainsKey("players")) return;
                    var players = json.Value<JArray>("players");

                    foreach (var player in players!)
                    {
                        var steamId = player["SteamId"]!.Value<string>();

                        // Update Vac and Limited in AppData.SteamUsers
                        var su = _appData.SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
                        if (su == null) continue;

                        su.Vac = player["VACBanned"]!.Value<bool>();
                        su.Limited = player["CommunityBanned"]!.Value<bool>();

                    }
                }
                catch (JsonReaderException e)
                {
                    Console.WriteLine(e);
                    throw;
                }
            }

            SaveVacInfo();
        }

        /// <summary>
        /// After querying Steam API, check the response here to see if the Web API key is valid
        /// </summary>
        /// <returns>True if key is valid. False if not, and action should be cancelled</returns>
        private async Task<bool> IsSteamApiKeyValid(string jsResponse)
        {
            if (!jsResponse.StartsWith("<html>")) return true;

            // Error: Key was likely invalid.
            await _generalFuncs.ShowToast("error", _lang["Toast_SteamWebKeyInvalid"],
                renderTo: "toastarea");

            SteamWebApiKey = "";
            SaveSettings();
            SteamWebApiWasReset = true;
            return false;

        }

        /// <summary>
        /// Checks for SteamVACCache.json, and assuming all info is there: Updates AppSettings.SteamUsers.
        /// Otherwise it updates all VAC/Limited info from Web API.
        /// </summary>
        private async Task WebApiPrepareBans()
        {
            _ = _generalFuncs.DeletedOutdatedFile(VacCacheFile, ImageExpiryTime);
            if (File.Exists(VacCacheFile))
            {
                var vsList = JsonConvert.DeserializeObject<List<VacStatus>>(Globals.ReadAllText(VacCacheFile));
                if (vsList != null && vsList.Count == _appData.SteamUsers.Count)
                {
                    foreach (var vs in vsList)
                    {
                        var su = _appData.SteamUsers.FirstOrDefault(x => x.SteamId == vs.SteamId);
                        if (su == null) continue;

                        su.Vac = vs.Vac;
                        su.Limited = vs.Ltd;
                    }

                    return;
                }
            }

            // File doesn't exist, has an error or has a different number of users > Refresh all vac info.
            await WebApiGetBans();
        }



        /// <summary>
        /// Restart Steam with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="steamId">(Optional) User's SteamID</param>
        /// <param name="ePersonaState">(Optional) Persona state for user [0: Offline, 1: Online...]</param>
        /// <param name="args">Starting arguments</param>
        public async Task SwapSteamAccounts(string steamId = "", int ePersonaState = -1, string args = "")
        {
            Globals.DebugWriteLine($@"[Func:Steam\Steam.SwapSteamAccounts] Swapping to: hidden. ePersonaState={ePersonaState}");
            if (steamId != "" && !VerifySteamId(steamId))
            {
                return;
            }

            await _appData.InvokeVoidAsync("updateStatus", _lang["Status_ClosingPlatform", new { platform = "Steam" }]);
            if (!await _generalFuncs.CloseProcesses(Processes, ClosingMethod))
            {
                if (Globals.IsAdministrator)
                    await _appData.InvokeVoidAsync("updateStatus", _lang["Status_ClosingPlatformFailed", new { platform = "Steam" }]);
                else
                {
                    await _generalFuncs.ShowToast("error", _lang["Toast_RestartAsAdmin"], _lang["Failed"], "toastarea");
                    _modalData.ShowModal("confirm", ExtraArg.RestartAsAdmin);
                }
                return;
            }

            if (OperatingSystem.IsWindows()) await UpdateLoginUsers(steamId, ePersonaState);

            await _appData.InvokeVoidAsync("updateStatus", _lang["Status_StartingPlatform", new { platform = "Steam" }]);
            if (AutoStart)
            {
                if (StartSilent) args += " -silent";

                if (Globals.StartProgram(Exe(), Admin, args, StartingMethod))
                    await _generalFuncs.ShowToast("info",
                        _lang["Status_StartingPlatform", new { platform = "Steam" }], renderTo: "toastarea");
                else
                    await _generalFuncs.ShowToast("error",
                        _lang["Toast_StartingPlatformFailed", new { platform = "Steam" }], renderTo: "toastarea");
            }

            if (AutoStart && _appSettings.MinimizeOnSwitch) await _appData.InvokeVoidAsync("hideWindow");

            NativeFuncs.RefreshTrayArea();
            await _appData.InvokeVoidAsync("updateStatus", _lang["Done"]);
            _appStats.IncrementSwitches("Steam");

            try
            {
                LastAccSteamId = _appData.SteamUsers.Where(x => x.SteamId == steamId).ToList()[0].SteamId;
                LastAccTimestamp = Globals.GetUnixTimeInt();
                if (LastAccSteamId != "")
                    await _accountFuncs.SetCurrentAccount(LastAccSteamId);
            }
            catch (Exception)
            {
                //
            }
        }

        /// <summary>
        /// Verify whether input Steam64ID is valid or not
        /// </summary>
        public static bool VerifySteamId(string steamId)
        {
            Globals.DebugWriteLine($@"[Func:Steam\Steam.VerifySteamId] Verifying SteamID: {steamId.Substring(steamId.Length - 4, 4)}");
            const long steamIdMin = 0x0110000100000001;
            const long steamIdMax = 0x01100001FFFFFFFF;
            if (!IsDigitsOnly(steamId) || steamId.Length != 17) return false;
            // Size check: https://stackoverflow.com/questions/33933705/steamid64-minimum-and-maximum-length#40810076
            var steamIdVal = double.Parse(steamId);
            return steamIdVal is > steamIdMin and < steamIdMax;
        }
        private static bool IsDigitsOnly(string str) => str.All(c => c is >= '0' and <= '9');
        #endregion

        #region STEAM_MANAGEMENT
        /// <summary>
        /// Updates loginusers and registry to select an account as "most recent"
        /// </summary>
        /// <param name="selectedSteamId">Steam ID64 to switch to</param>
        /// <param name="pS">[PersonaState]0-7 custom persona state [0: Offline, 1: Online...]</param>
        [SupportedOSPlatform("windows")]
        public async Task UpdateLoginUsers(string selectedSteamId, int pS)
        {
            Globals.DebugWriteLine($@"[Func:Steam\Steam.UpdateLoginUsers] Updating loginusers: selectedSteamId={(selectedSteamId.Length > 0 ? selectedSteamId.Substring(selectedSteamId.Length - 4, 4) : "")}, pS={pS}");
            var userAccounts = await GetSteamUsers(LoginUsersVdf());
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            await _appData.InvokeVoidAsync("updateStatus", _lang["Status_UpdatingFile", new { file = "loginusers.vdf" }]);
            var tempFile = LoginUsersVdf() + "_temp";
            Globals.DeleteFile(tempFile);

            // MostRec is "00" by default, just update the one that matches SteamID.
            try
            {
                userAccounts.Where(x => x.SteamId == selectedSteamId).ToList().ForEach(u =>
                {
                    u.MostRec = "1";
                    u.RememberPass = "1";
                    u.OfflineMode = pS == -1 ? u.OfflineMode : pS > 1 ? "0" : pS == 1 ? "0" : "1";
                    // u.OfflineMode: Set ONLY if defined above
                    // If defined & > 1, it's custom, therefor: Online
                    // Otherwise, invert [0 == Offline => Online, 1 == Online => Offline]
                });
            }
            catch (InvalidOperationException)
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_MissingUserId"]);
            }
            //userAccounts.Single(x => x.SteamId == selectedSteamId).MostRec = "1";

            // Save updated loginusers.vdf
            await SaveSteamUsersIntoVdf(userAccounts);

            // -----------------------------------
            // - Update localconfig.vdf for user -
            // -----------------------------------
            if (pS != -1) SetPersonaState(selectedSteamId, pS); // Update persona state, if defined above.

            SteamUserAcc user = new() { AccName = "" };
            try
            {
                if (selectedSteamId != "")
                    user = userAccounts.Single(x => x.SteamId == selectedSteamId);
            }
            catch (InvalidOperationException)
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_MissingUserId"]);
            }
            // -----------------------------------
            // --------- Manage registry ---------
            // -----------------------------------
            /*
            ------------ Structure ------------
            HKEY_CURRENT_USER\Software\Valve\Steam\
                --> AutoLoginUser = username
                --> RememberPassword = 1
            */
            await _appData.InvokeVoidAsync("updateStatus", _lang["Status_UpdatingRegistry"]);
            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam");
            key?.SetValue("AutoLoginUser", user.AccName); // Account name is not set when changing user accounts from launch arguments (part of the viewmodel). -- Can be "" if no account
            key?.SetValue("RememberPassword", 1);

            // -----------------------------------
            // ------Update Tray users list ------
            // -----------------------------------
            if (selectedSteamId != "")
                Globals.AddTrayUser("Steam", "+s:" + user.SteamId, TrayAccName ? user.AccName : GetName(user), TrayAccNumber);
        }

        /// <summary>
        /// Save updated list of Steamuser into loginusers.vdf, in vdf format.
        /// </summary>
        /// <param name="userAccounts">List of Steamuser to save into loginusers.vdf</param>
        public async Task SaveSteamUsersIntoVdf(List<SteamUserAcc> userAccounts)
        {
            Globals.DebugWriteLine($@"[Func:Steam\Steam.SaveSteamUsersIntoVdf] Saving updated loginusers.vdf. Count: {userAccounts.Count}");
            // Convert list to JObject list, ready to save into vdf.
            var outJObject = new JObject();
            foreach (var ua in userAccounts)
            {
                outJObject[ua.SteamId] = (JObject)JToken.FromObject(ua);
            }

            // Write changes to files.
            var tempFile = LoginUsersVdf() + "_temp";
            await File.WriteAllTextAsync(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            if (!File.Exists(tempFile))
            {
                File.Replace(tempFile, LoginUsersVdf(), LoginUsersVdf() + "_last");
                return;
            }
            try
            {
                // Let's try break this down, as some users are having issues with the above function.
                // Step 1: Backup
                if (File.Exists(LoginUsersVdf()))
                {
                    File.Copy(LoginUsersVdf(), LoginUsersVdf() + "_last", true);
                }

                // Step 2: Write new info
                await File.WriteAllTextAsync(LoginUsersVdf(), @"""users""" + Environment.NewLine + outJObject.ToVdf());
            }
            catch (Exception ex)
            {
                Globals.WriteToLog("Failed to swap Steam users! Could not create temp loginusers.vdf file, and replace original using workaround! Contact TechNobo.", ex);
                await _generalFuncs.ShowToast("error", _lang["CouldNotFindX", new { x = tempFile }]);
            }
        }

        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        /// <returns>Whether files were deleted or not</returns>
        public async Task ClearImages()
        {
            Globals.DebugWriteLine(@"[Func:Steam\Steam.ClearImages] Clearing images.");
            if (!Directory.Exists(SteamImagePath))
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_CantClearImages"], _lang["Error"], "toastarea");
            }
            Globals.DeleteFiles(SteamImagePath);
            // Reload page, then display notification using a new thread.
            _appData.NavigateTo(
                $"/steam/?cacheReload&toast_type=success&toast_title={Uri.EscapeDataString(_lang["Success"])}&toast_message={Uri.EscapeDataString(_lang["Toast_ClearedImages"])}", true);
        }

        /// <summary>
        /// Sets whether the user is invisible or not
        /// </summary>
        /// <param name="steamId">SteamID of user to update</param>
        /// <param name="ePersonaState">Persona state enum for user (0-7)</param>
        public void SetPersonaState(string steamId, int ePersonaState)
        {
            Globals.DebugWriteLine($@"[Func:Steam\Steam.SetPersonaState] Setting persona state for: {steamId.Substring(steamId.Length - 4, 4)}, To: {ePersonaState}");
            // Values:
            // 0: Offline, 1: Online, 2: Busy, 3: Away, 4: Snooze, 5: Looking to Trade, 6: Looking to Play, 7: Invisible
            var id32 = new SteamIdConvert(steamId).Id32; // Get SteamID
            var localConfigFilePath = Path.Join(FolderPath, "userdata", id32, "config", "localconfig.vdf");
            if (!File.Exists(localConfigFilePath)) return;
            var localConfigText = Globals.ReadAllText(localConfigFilePath); // Read relevant localconfig.vdf

            // Find index of range needing to be changed.
            var positionOfVar = localConfigText.IndexOf("ePersonaState", StringComparison.Ordinal); // Find where the variable is being set
            if (positionOfVar == -1) return;
            var indexOfBefore = localConfigText.IndexOf(":", positionOfVar, StringComparison.Ordinal) + 1; // Find where the start of the variable's value is
            var indexOfAfter = localConfigText.IndexOf(",", positionOfVar, StringComparison.Ordinal); // Find where the end of the variable's value is

            // The variable is now in-between the above numbers. Remove it and insert something different here.
            var sb = new StringBuilder(localConfigText);
            _ = sb.Remove(indexOfBefore, indexOfAfter - indexOfBefore);
            _ = sb.Insert(indexOfBefore, ePersonaState);
            localConfigText = sb.ToString();

            // Output
            File.WriteAllText(localConfigFilePath, localConfigText);
        }

        /// <summary>
        /// Returns string representation of Steam ePersonaState int
        /// </summary>
        /// <param name="ePersonaState">integer state to return string for</param>
        public string PersonaStateToString(int ePersonaState)
        {
            return ePersonaState switch
            {
                -1 => "",
                0 => _lang["Offline"],
                1 => _lang["Online"],
                2 => _lang["Busy"],
                3 => _lang["Away"],
                4 => _lang["Snooze"],
                5 => _lang["LookingToTrade"],
                6 => _lang["LookingToPlay"],
                7 => _lang["Invisible"],
                _ => _lang["Unrecognized_EPersonaState"]
            };
        }
        #endregion

        #region STEAM_GAME_MANAGEMENT
        /// <summary>
        /// Copy settings from currently logged in account to selected game and account
        /// </summary>
        public async Task CopySettingsFrom(string gameId)
        {
            var destSteamId = GetCurrentAccountId(true);
            if (!VerifySteamId(_appData.SelectedAccountId) || !VerifySteamId(destSteamId))
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_NoValidSteamId"], _lang["Failed"],
                    "toastarea");
                return;
            }
            if (destSteamId == _appData.SelectedAccountId)
            {
                await _generalFuncs.ShowToast("info", _lang["Toast_SameAccount"], _lang["Failed"],
                    "toastarea");
                return;
            }

            var sourceSteamId32 = new SteamIdConvert(_appData.SelectedAccountId).Id32;
            var destSteamId32 = new SteamIdConvert(destSteamId).Id32;
            var sourceFolder = Path.Join(FolderPath, $"userdata\\{sourceSteamId32}\\{gameId}");
            var destFolder = Path.Join(FolderPath, $"userdata\\{destSteamId32}\\{gameId}");
            if (!Directory.Exists(sourceFolder))
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_NoFindSteamUserdata"], _lang["Failed"],
                    "toastarea");
                return;
            }

            if (Directory.Exists(destFolder))
            {
                // Backup the account's data you're copying to
                var toAccountBackup = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{destSteamId32}\\{gameId}");
                if (Directory.Exists(toAccountBackup)) Directory.Delete(toAccountBackup, true);
                Globals.CopyDirectory(destFolder, toAccountBackup, true);
                Directory.Delete(destFolder, true);
            }
            Globals.CopyDirectory(sourceFolder, destFolder, true);
            await _generalFuncs.ShowToast("success", _lang["Toast_SettingsCopied"], _lang["Success"], "toastarea");
        }


        public async Task RestoreSettingsTo(string gameId)
        {
            if (!VerifySteamId(_appData.SelectedAccountId)) return;
            var steamId32 = new SteamIdConvert(_appData.SelectedAccountId).Id32;
            var backupFolder = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{steamId32}\\{gameId}");

            var folder = Path.Join(FolderPath, $"userdata\\{steamId32}\\{gameId}");
            if (!Directory.Exists(backupFolder))
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_NoFindGameBackup"], _lang["Failed"],
                    "toastarea");
                return;
            }
            if (Directory.Exists(folder)) Directory.Delete(folder, true);
            Globals.CopyDirectory(backupFolder, folder, true);
            await _generalFuncs.ShowToast("success", _lang["Toast_GameDataRestored"], _lang["Success"], "toastarea");
        }

        public async Task BackupGameData(string gameId)
        {
            var steamId32 = new SteamIdConvert(_appData.SelectedAccountId).Id32;
            var sourceFolder = Path.Join(FolderPath, $"userdata\\{steamId32}\\{gameId}");
            if (!VerifySteamId(_appData.SelectedAccountId) || !Directory.Exists(sourceFolder))
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_NoFindGameData"], _lang["Failed"],
                    "toastarea");
                return;
            }

            var destFolder = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{steamId32}\\{gameId}");
            if (Directory.Exists(destFolder)) Directory.Delete(destFolder, true);

            Globals.CopyDirectory(sourceFolder, destFolder, true);
            await _generalFuncs.ShowToast("success", _lang["Toast_GameBackupDone", new { folderLocation = destFolder }], _lang["Success"], "toastarea");
        }
        #endregion
    }

    /// <summary>
    /// Simple class for storing info related to Steam account, for switching and displaying.
    /// </summary>
    public class SteamUserAcc
    {
        [JsonIgnore] public string SteamId { get; set; }
        [JsonProperty("AccountName", Order = 0)] public string AccName { get; set; }
        [JsonProperty("PersonaName", Order = 1)] public string Name { get; set; }
        [JsonProperty("RememberPassword", Order = 2)] public string RememberPass = "1"; // Should always be 1
        [JsonProperty("mostrecent", Order = 3)] public string MostRec = "0";
        [JsonProperty("Timestamp", Order = 4)] public string LastLogin { get; set; }
        [JsonProperty("WantsOfflineMode", Order = 5)] public string OfflineMode = "0";
        [JsonIgnore] public string ImageDownloadUrl { get; set; }
        [JsonIgnore] public string ImgUrl { get; set; }
        [JsonIgnore] public bool BanInfoLoaded { get; set; }
        [JsonIgnore] public bool Vac { get; set; }
        // Either Limited or Community banned status (when using Steam Web API)
        [JsonIgnore] public bool Limited { get; set; }

    }
}
