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
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Gameloop.Vdf;
using Gameloop.Vdf.JsonConverter;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.Shared.Modal;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes.Steam
{
    public class SteamContextMenu
    {
        [Inject] private NewLang Lang { get; set; }
        [Inject] private Modals Modals { get; set; }
        [Inject] private SteamSettings SteamSettings { get; set; }
        [Inject] IAppState AppState { get; set; }
        [Inject] Toasts Toasts { get; set; }

        public List<string> InstalledGames { get; set; }
        public Dictionary<string, string> AppIds { get; set; }

        public static readonly string SteamAppsListPath =
            Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsFullListCache.json");

        public static readonly string SteamAppsUserCache =
            Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsUser.json");

        public readonly ObservableCollection<MenuItem> Menu = new();

        public SteamContextMenu()
        {
            LoadInstalledGames();
            AppIds = LoadAppNames();
        }

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
                        Children = new List<MenuItem>
                        {
                            new()
                            {
                                Text = Lang["Context_Game_CopySettingsFrom"],
                                MenuAction = async () => await SteamSwitcherFuncs.CopySettingsFrom(gameId)
                            },
                            new()
                            {
                                Text = Lang["Context_Game_RestoreSettingsTo"],
                                MenuAction = async () => await SteamSwitcherFuncs.RestoreSettingsTo(gameId)
                            },
                            new()
                            {
                                Text = Lang["Context_Game_BackupData"],
                                MenuAction = async () => await SteamSwitcherFuncs.BackupGameData(gameId)
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
                new("Context_SwapTo", new Action(async () => await AppFuncs.SwapToAccount())),
                new("Context_LoginAsSubmenu", new Tuple<string, object>[]
                    {
                        new("Invisible", new Action(async () => await AppFuncs.SwapToAccount(7))),
                        new("Offline", new Action(async () => await AppFuncs.SwapToAccount(0))),
                        new("Online", new Action(async () => await AppFuncs.SwapToAccount(1))),
                        new("Busy", new Action(async () => await AppFuncs.SwapToAccount(2))),
                        new("Away", new Action(async () => await AppFuncs.SwapToAccount(3))),
                        new("Snooze", new Action(async () => await AppFuncs.SwapToAccount(4))),
                        new("LookingToTrade", new Action(async () => await AppFuncs.SwapToAccount(5))),
                        new("LookingToPlay", new Action(async () => await AppFuncs.SwapToAccount(6))),
                    }
                ),
                new("Context_CopySubmenu", new Tuple<string, object>[]
                {
                    new("Context_CopyProfileSubmenu", new Tuple<string, object>[]
                    {
                        new("Context_CommunityUrl",
                            new Action(async () =>
                                await AppFuncs.CopyText(
                                    $"https://steamcommunity.com/profiles/{AppState.Switcher.SelectedAccountId}"))),
                        new("Context_CommunityUsername",
                            new Action(async () => await AppFuncs.CopyText(AppState.Switcher.SelectedAccount.DisplayName))),
                        new("Context_LoginUsername",
                            new Action(async () => await AppFuncs.CopyText(AppState.Switcher.SelectedAccount.LoginUsername))),
                    }),
                    new("Context_CopySteamIdSubmenu", new Tuple<string, object>[]
                    {
                        new("Context_Steam_Id",
                            new Action(async () =>
                                await AppFuncs.CopyText(new SteamIdConvert(AppState.Switcher.SelectedAccountId).Id))),
                        new("Context_Steam_Id3",
                            new Action(async () =>
                                await AppFuncs.CopyText(new SteamIdConvert(AppState.Switcher.SelectedAccountId).Id3))),
                        new("Context_Steam_Id32",
                            new Action(async () =>
                                await AppFuncs.CopyText(new SteamIdConvert(AppState.Switcher.SelectedAccountId).Id32))),
                        new("Context_Steam_Id64",
                            new Action(async () =>
                                await AppFuncs.CopyText(new SteamIdConvert(AppState.Switcher.SelectedAccountId).Id64))),
                    }),
                    new("Context_CopyOtherSubmenu", new Tuple<string, object>[]
                    {
                        new("SteamRep",
                            new Action(async () =>
                                await AppFuncs.CopyText($"https://steamrep.com/search?q={AppState.Switcher.SelectedAccountId}"))),
                        new("SteamID.uk",
                            new Action(async () =>
                                await AppFuncs.CopyText($"https://steamid.uk/profile/{AppState.Switcher.SelectedAccountId}"))),
                        new("SteamID.io",
                            new Action(async () =>
                                await AppFuncs.CopyText($"https://steamid.io/lookup/{AppState.Switcher.SelectedAccountId}"))),
                        new("SteamIDFinder.com",
                            new Action(async () =>
                                await AppFuncs.CopyText(
                                    $"https://steamidfinder.com/lookup/{AppState.Switcher.SelectedAccountId}"))),
                    }),
                }),
                new("Context_CreateShortcut", new Tuple<string, object>[]
                {
                    new("OnlineDefault", new Action(async () => await GeneralInvocableFuncs.CreateShortcut())),
                    new("Invisible", new Action(async () => await GeneralInvocableFuncs.CreateShortcut(":7"))),
                    new("Offline", new Action(async () => await GeneralInvocableFuncs.CreateShortcut(":0"))),
                    new("Busy", new Action(async () => await GeneralInvocableFuncs.CreateShortcut(":2"))),
                    new("Away", new Action(async () => await GeneralInvocableFuncs.CreateShortcut(":3"))),
                    new("Snooze", new Action(async () => await GeneralInvocableFuncs.CreateShortcut(":4"))),
                    new("LookingToTrade", new Action(async () => await GeneralInvocableFuncs.CreateShortcut(":5"))),
                    new("LookingToPlay", new Action(async () => await GeneralInvocableFuncs.CreateShortcut(":6"))),
                }),
                new("Forget", new Action(async () => await AppFuncs.ForgetAccount())),
                new("Notes", new Action(() => Modals.ShowModal("notes"))),
                new("Context_ManageSubmenu", new[]
                {
                    gameData is not null
                        ? new Tuple<string, object>("Context_GameDataSubmenu", gameData)
                        : null,
                    BasicStats.PlatformHasAnyGames("Steam")
                        ? new Tuple<string, object>("Context_ManageGameStats",
                            new Action(Modals.ShowGameStatsSelectorModal))
                        : null,
                    new("Context_ChangeImage", new Action(ModalFuncs.ShowChangeAccImageModal)),
                    new("Context_Steam_OpenUserdata", new Action(SteamOpenUserdata)),
                    new("Context_ChangeName", new Action(ModalFuncs.ShowChangeUsernameModal))
                })
            });

            Menu.AddRange(menuBuilder.Result());
        }
        public void SteamOpenUserdata()
        {
            var steamId32 = new SteamIdConvert(AppState.Switcher.SelectedAccountId);
            var folder = Path.Join(SteamSettings.FolderPath, $"userdata\\{steamId32.Id32}");
            if (Directory.Exists(folder))
                _ = Process.Start("explorer.exe", folder);
            else
                Toasts.ShowToastLang(ToastType.Error, "Failed", "Toast_NoFindSteamUserdata");
        }
        public void LoadInstalledGames()
        {
            List<string> gameIds;
            try
            {
                var libraryVdf = VdfConvert.Deserialize(File.ReadAllText(SteamSettings.LoginUsersVdf));
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
            InstalledGames = gameIds;
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
            try
            {
                // Check if all the IDs we need are in the cache, i.e. the user has not installed any new games.
                if (File.Exists(cacheFilePath))
                {
                    var cachedAppIds = ParseSteamAppsText(File.ReadAllText(cacheFilePath));
                    if (InstalledGames.All(id => cachedAppIds.ContainsKey(id)))
                    {
                        return cachedAppIds;
                    }
                }

                // If the cache is missing or incomplete, fetch app Ids from Steam's API
                appIds =
                    (from game in ParseSteamAppsText(FetchSteamAppsData())
                     where InstalledGames.Contains(game.Key)
                     select game)
                    .ToDictionary(game => game.Key, game => game.Value);

                // Downloading app list for the first time.
                if (appIds.Count == 0) return appIds;

                // Add any missing games as just the appid. These can include games/apps not on steam (developer Steam accounts), or otherwise removed games from Steam.
                if (appIds.Count != InstalledGames.Count)
                {
                    foreach (var g in (from game in InstalledGames where !appIds.ContainsKey(game) select game))
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
        public async Task DownloadSteamAppsData()
        {
            await GeneralInvocableFuncs.ShowToast("info", Lang["Toast_Steam_DownloadingAppIds"], renderTo: "toastarea");

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

            await GeneralInvocableFuncs.ShowToast("info", Lang["Toast_Steam_DownloadingAppIdsComplete"], renderTo: "toastarea");
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

        /// <summary>
        /// Fetches the names corresponding to each game ID from Valve's API.
        /// </summary>
        private static string FetchSteamAppsData()
        {
            // TODO: Copy the GitHub repo that downloads the latest apps, and shares as XML and CSV. Then remove those, and replace it with compressing with 7-zip. Download the latest 7-zip archive here, decompress then read. It takes literally ~1.5MB instead of ~8MB. HUGE saving for super slow internet.
            return File.Exists(SteamAppsListPath) ? File.ReadAllText(SteamAppsListPath) : "";
        }
    }
}
