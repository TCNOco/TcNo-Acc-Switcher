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
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Runtime.Versioning;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using Gameloop.Vdf;
using Gameloop.Vdf.JsonConverter;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;


namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherFuncs
    {
        private static readonly Lang Lang = Lang.Instance;

        #region STEAM_SWITCHER_MAIN
        // Checks if Steam path set properly, and can load.
        public static bool SteamSettingsValid() => SteamSettings.LoginUsersVdf() != "RESET_PATH";

        private static string GetName(Index.Steamuser su) => string.IsNullOrWhiteSpace(su.Name) ? su.AccName : su.Name;

        /// <summary>
        /// Main function for Steam Account Switcher. Run on load.
        /// Collects accounts from Steam's loginusers.vdf
        /// Prepares images and VAC/Limited status
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async Task LoadProfiles()
        {
            if (AppData.SteamLoadingProfiles) return;
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.LoadProfiles] Loading Steam profiles");
            AppData.SteamLoadingProfiles = true;

            AppData.SteamUsers = GetSteamUsers(SteamSettings.LoginUsersVdf());

            var orderFilePath = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\order.json");

            // Order
            if (File.Exists(orderFilePath))
            {
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync(orderFilePath).ConfigureAwait(false));
                if (savedOrder != null)
                {
                    var index = 0;
                    if (savedOrder is { Count: > 0 })
                        foreach (var acc in from i in savedOrder where AppData.SteamUsers.Any(x => x.SteamId == i) select AppData.SteamUsers.Single(x => x.SteamId == i))
                        {
                            _ = AppData.SteamUsers.Remove(acc);
                            AppData.SteamUsers.Insert(Math.Min(index, AppData.SteamUsers.Count), acc);
                            index++;
                        }
                }
            }

            // If Steam Web API key to be used instead
            if (!AppSettings.OfflineMode && SteamSettings.SteamWebApiKey != "")
            {
                // Handle all image downloads
                await WebApiPrepareImages();
                WebApiPrepareBans();

                // Key was fine? Continue. If not, the non-api method will be used.
                if (!SteamSettings.SteamWebApiWasReset)
                {
                    SteamSettings.Accounts.Clear();
                    foreach (var su in AppData.SteamUsers)
                    {
                        InsertAccount(su);
                    }
                }
            }

            // If Steam Web API key was not used, or was reset (encountered an error)
            if (SteamSettings.SteamWebApiKey == "" || SteamSettings.SteamWebApiWasReset)
            {
                SteamSettings.SteamWebApiWasReset = false;

                // Load cached ban info
                LoadCachedBanInfo();
                SteamSettings.Accounts.Clear();

                foreach (var su in AppData.SteamUsers)
                {
                    // If not cached: Get ban status and images
                    // If cached: Just get images
                    PrepareProfile(su.SteamId, !su.BanInfoLoaded); // Get ban status as well if was not loaded (missing from cache)

                    InsertAccount(su);
                }

                SaveVacInfo();
            }

            GenericFunctions.FinaliseAccountList();
            AppStats.SetAccountCount("Steam", AppData.SteamUsers.Count);
            AppData.SteamLoadingProfiles = false;
        }

        private static void InsertAccount(Index.Steamuser su)
        {
            var account = new Account
            {
                Platform = "Steam",
                AccountId = su.SteamId,
                DisplayName = GeneralFuncs.EscapeText(GetName(su)),
                Classes = new Account.ClassCollection()
                {
                    Image = (SteamSettings.ShowVac && su.Vac ? " status_vac" : "") + (SteamSettings.ShowLimited && su.Limited ? " status_limited" : ""),
                    Line0 = "streamerCensor",
                    Line2 = "streamerCensor steamId",
                    Line3 = "lastLogin"
                },
                UserStats = BasicStats.GetUserStatsAllGamesMarkup("Steam", su.SteamId),
                ImagePath = su.ImgUrl,
                Line0 = SteamSettings.ShowAccUsername ? su.AccName : "",
                Line2 = su.SteamId,
                Line3 = UnixTimeStampToDateTime(su.LastLogin)
            };
            SteamSettings.Accounts.Add(account);
        }


        /// <summary>
        /// This relies on Steam updating loginusers.vdf. It could go out of sync assuming it's not updated reliably. There is likely a better way to do this.
        /// I am avoiding using the Steam API because it's another DLL to include, but is the next best thing - I assume.
        /// </summary>
        public static string GetCurrentAccountId()
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.GetCurrentAccountId]");
            try
            {
                // Refreshing the list of SteamUsers doesn't help here when switching, as the account list is not updated by Steam just yet.
                Index.Steamuser mostRecent = null;
                foreach (var su in AppData.SteamUsers)
                {
                    _ = int.TryParse(su.LastLogin, out int last);

                    _ = int.TryParse(mostRecent?.LastLogin, out int recent);

                    if (mostRecent == null || last > recent)
                        mostRecent = su;
                }

                if (mostRecent is null) return "";

                if (int.TryParse(mostRecent?.LastLogin, out var mrTimestamp) && SteamSettings.LastAccTimestamp > mrTimestamp)
                    return SteamSettings.LastAccName;

                return mostRecent.SteamId ?? "";
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
        /// <returns>List of Steamuser classes, from loginusers.vdf</returns>
        public static List<Index.Steamuser> GetSteamUsers(string loginUserPath)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.GetSteamUsers] Getting list of Steam users from {loginUserPath}");
            _ = Directory.CreateDirectory("wwwroot/img/profiles");

            if (LoadFromVdf(loginUserPath, out var userAccounts)) return userAccounts;

            // Didn't work, try last file, if exists.
            var lastVdf = loginUserPath.Replace(".vdf", ".vdf_last");
            if (!File.Exists(lastVdf) || !LoadFromVdf(lastVdf, out userAccounts)) return new List<Index.Steamuser>();

            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_Steam_VdfLast"], Lang["Toast_PartiallyFixed"], "toastarea", 10000);
            return userAccounts;
        }

        /// <summary>
        /// Fetches the names corresponding to each game ID from Valve's API.
        /// </summary>
        private static string FetchSteamAppsData()
        {
            // TODO: Copy the GitHub repo that downloads the latest apps, and shares as XML and CSV. Then remove those, and replace it with compressing with 7-zip. Download the latest 7-zip archive here, decompress then read. It takes literally ~1.5MB instead of ~8MB. HUGE saving for super slow internet.
            return File.Exists(SteamSettings.SteamAppsListPath) ? File.ReadAllText(SteamSettings.SteamAppsListPath) : "";
        }

        public static void DownloadSteamAppsData()
        {
            if (AppSettings.OfflineMode) {
                Globals.DebugWriteLine($@"Error downloading Steam app list: OFFLINE MODE");
                return;
            }

            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_Steam_DownloadingAppIds"], renderTo: "toastarea");

            try
            {
                // Save to file
                var file = new FileInfo(SteamSettings.SteamAppsListPath);
                if (file.Exists) file.Delete();
                if (Globals.ReadWebUrl("https://api.steampowered.com/ISteamApps/GetAppList/v2/", out var appList))
                    File.WriteAllText(file.FullName, appList);
                else
                    throw new Exception("Failed to download Steam apps list.");
            }
            catch (Exception e)
            {
                Globals.DebugWriteLine($@"Error downloading Steam app list: {e}");
            }

            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_Steam_DownloadingAppIdsComplete"], renderTo: "toastarea");
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
                foreach (var app in json["applist"]["apps"])
                {
                    if (appIds.ContainsKey(app["appid"].Value<string>())) continue;
                    appIds.Add(app["appid"].Value<string>(), app["name"].Value<string>());
                }
            }
            catch (Exception e)
            {
                Globals.DebugWriteLine($@"Error parsing Steam app list: {e}");
            }
            return appIds;
        }

        public static Dictionary<string, string> LoadAppNames()
        {
            // Check if cached Steam AppId list is downloaded
            // If not, skip. Download is handled in a background task.
            if (!File.Exists(SteamSettings.SteamAppsListPath))
            {
                // Create the folder if does not exist
                var folder = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\");
                Directory.CreateDirectory(folder);

                // Download Steam AppId list if not already.
                Task.Run(DownloadSteamAppsData).ContinueWith(_ =>
                {
                    var names = LoadAppNames();
                    foreach (var kv in names)
                    {
                        try
                        {
                            SteamSettings.AppIds.Value.Add(kv.Key, kv.Value);
                        }
                        catch (Exception e)
                        {
                            Console.WriteLine(e.ToString());
                        }
                    }
                    SteamSettings.BuildContextMenu();
                });
                return new Dictionary<string, string>();
            }

            var cacheFilePath = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\AppIdsUser.json");
            var appIds = new Dictionary<string, string>();
            var gameList = SteamSettings.InstalledGames.Value;
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

                // Add any missing games as just the appid. These can include games/apps not on steam (developer Steam accounts), or otherwise removed games from Steam.
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
        public static List<string> LoadInstalledGames()
        {
            List<string> gameIds;
            try
            {
                var libraryFile = Path.Join(SteamSettings.FolderPath, "steamapps\\libraryfolders.vdf");
                var libraryVdf = VdfConvert.Deserialize(File.ReadAllText(libraryFile));
                var library = new JObject { libraryVdf.ToJson() };
                gameIds = library["libraryfolders"]
                    .SelectMany(folder => (folder.First["apps"] as JObject)
                        .Properties()
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
        private static bool LoadFromVdf(string vdf, out List<Index.Steamuser> userAccounts)
        {
            userAccounts = new List<Index.Steamuser>();

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

                            userAccounts.Add(new Index.Steamuser
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
                            Globals.WriteToLog("Could not import Steam user. Please send your loginusers.vdf file to TroubleChute for analysis.");
                        }
                    }
                }
            }
            catch (FileNotFoundException)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_Steam_FailedLoginusers"], Lang["NotFound"], "toastarea");
                return false;
            }
            catch (AggregateException)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_Steam_FailedLoginusers"], Lang["Error"], "toastarea");
                return false;
            }
            catch (Exception)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_Steam_FailedLoginusers"], Lang["Error"], "toastarea");
                return false;
            }


            return true;
        }

        private static string VerifyVdfText(string loginUserPath)
        {
            var original = Globals.ReadAllText(loginUserPath);
            var vdf = original;
            //// Replaces double quotes, sometimes added by mistake (?) with single, as they should be.
            //vdf = vdf.Replace("\"\"", "\"");

            // Logic can be improved here. Currently the above causes issues, whereas before this was nessecary... Very flaky.

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
        public static void LoadCachedBanInfo()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.LoadCachedBanInfo]");
            _ = GeneralFuncs.DeletedOutdatedFile(SteamSettings.VacCacheFile, SteamSettings.ImageExpiryTime);
            if (!File.Exists(SteamSettings.VacCacheFile)) return;

            // Load list of banStatus
            var banInfoList = JsonConvert.DeserializeObject<List<VacStatus>>(Globals.ReadAllText(SteamSettings.VacCacheFile));

            if (banInfoList == null) return;
            foreach (var su in AppData.SteamUsers)
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
        public static void SaveVacInfo()
        {
            var vsList = AppData.SteamUsers.Select(su => new VacStatus
                {
                    SteamId = su.SteamId,
                    Vac = su.Vac,
                    Ltd = su.Limited
                })
                .ToList();

            _ = Directory.CreateDirectory(Path.GetDirectoryName(SteamSettings.VacCacheFile) ?? string.Empty);
            File.WriteAllText(SteamSettings.VacCacheFile, JsonConvert.SerializeObject(vsList));
        }

        /// <summary>
        /// Converts Unix Timestamp string to DateTime
        /// </summary>
        public static string UnixTimeStampToDateTime(string stringUnixTimeStamp)
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
        public static void PrepareProfile(string steamId, bool noCache)
        {
            var su = AppData.SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
            if (su == null) return;
            Globals.DebugWriteLine(
                $@"[Func:Steam\SteamSwitcherFuncs.PrepareProfile] Preparing image and ban info for: {su.SteamId.Substring(su.SteamId.Length - 4, 4)}");
            _ = Directory.CreateDirectory(SteamSettings.SteamImagePath);

            if (!SteamSettings.CollectInfo) su.ImgUrl = "img/QuestionMark.jpg";

            var dlDir = $"{SteamSettings.SteamImagePath}{su.SteamId}.jpg";
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
                _ = GeneralFuncs.DeletedOutdatedFile(dlDir, SteamSettings.ImageExpiryTime);
                _ = GeneralFuncs.DeletedInvalidImage(dlDir);

                // 1.2 Clear old cached profile data
                _ = GeneralFuncs.DeletedOutdatedFile(cachedFile, 1);
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
                if (!File.Exists(cachedFile))
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_SteamCantLoadXml", new { steamId = su.SteamId }],
                        renderTo: "toastarea");
                    return;
                }

                Globals.DeleteFile(cachedFile);
                Globals.WriteToLog("The issue was the local XML file. It has been deleted and re-downloaded.");

                // Issue was caused by cached fil. Delete, and re-download.
                try
                {
                    profileXml.Load($"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
                }
                catch (Exception ex)
                {
                    Globals.WriteToLog("Failed to download profile XML info", ex);
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_SteamCantLoadXml", new { steamId = su.SteamId }],
                        renderTo: "toastarea");
                }
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
                    if (!AppSettings.OfflineMode)
                    {
                        Globals.DownloadFile(su.ImageDownloadUrl, dlDir);
                        su.ImgUrl = $"{SteamSettings.SteamImagePathHtml}{su.SteamId}.jpg";
                    }
                    else
                        su.ImgUrl = "img/QuestionMark.jpg";
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
                ? $"{SteamSettings.SteamImagePathHtml}{su.SteamId}.jpg"
                : "img/QuestionMark.jpg";
        }

        /// <summary>
        /// Gets ban status and image from input XML Document.
        /// </summary>
        /// <param name="profileXml">User's profile XML string</param>
        private static void ProcessSteamUserXml(XmlDocument profileXml)
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.XmlGetVacLimitedStatus] Get Ban status for Steam account.");

            // Read SteamID and select the correct Steam User to edit
            var steamId = profileXml.DocumentElement?.SelectNodes("/profile/steamID64")?[0]?.InnerText;
            if (steamId == null) return;
            var su = AppData.SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
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
                Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.XmlGetVacLimitedStatus] SUPPRESSED ERROR: NullReferenceException");
            }
        }

        /// <summary>
        /// Gets a list of profile image URLs for all profiles
        /// This replaces GetUserImageUrl, and does it for every SteamId at once
        /// </summary>
        /// <returns>Dictionary of [Profile image URL] = Destination file path</returns>
        private static Dictionary<string, string> WebApiGetImageList(IReadOnlyCollection<Index.Steamuser> steamUsers)
        {
            var images = new Dictionary<string, string>();
            // Web API can take up to 100 items at once.
            for (var i = 0; i < steamUsers.Count; i += 100)
            {
                var steamUsersGroup = steamUsers.Skip(i).Take(100);
                var steamIds = steamUsersGroup.Select(su => su.SteamId).ToList();
                var uri =
                    $"https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key={SteamSettings.SteamWebApiKey}&steamids={string.Join(',', steamIds)}";
                Globals.ReadWebUrl(uri, out var jsonString);
                if (!IsSteamApiKeyValid(jsonString)) return images;
                var json = JObject.Parse(jsonString);

                if (!json.ContainsKey("response")) return images;
                json = json.Value<JObject>("response");
                if (!json!.ContainsKey("players")) return images;
                var players = json.Value<JArray>("players");

                foreach (var player in players!)
                {
                    var steamId = player["steamid"]!.Value<string>();
                    var imageUrl = player["avatarfull"]!.Value<string>();
                    if (steamId != null && imageUrl != null) images.Add(imageUrl, $"{SteamSettings.SteamImagePath}/{steamId}.jpg");
                }
            }

            return images;
        }

        /// <summary>
        /// Download all missing or outdated Steam profile images, multi-threaded
        /// </summary>
        private static async Task WebApiPrepareImages()
        {
            // Create a queue of SteamUsers to download images for
            var queue = new List<Index.Steamuser>();

            foreach (var su in AppData.SteamUsers)
            {
                var dlDir = $"{SteamSettings.SteamImagePath}{su.SteamId}.jpg";
                // Delete outdated file, if it exists
                var wasDeleted = GeneralFuncs.DeletedOutdatedFile(dlDir, SteamSettings.ImageExpiryTime);
                // ... & invalid files
                wasDeleted = wasDeleted || GeneralFuncs.DeletedInvalidImage(dlDir);

                if (wasDeleted) queue.Add(su);
            }

            // Either return if queue is empty, or download queued images and then continue.
            if (queue.Count != 0)
            {
                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_DownloadingProfileData"], renderTo: "toastarea");
                await Globals.MultiThreadParallelDownloads(WebApiGetImageList(queue)).ConfigureAwait(true);
            }

            // Set the correct path
            foreach (var su in AppData.SteamUsers)
            {
                su.ImgUrl = $"{SteamSettings.SteamImagePathHtml}{su.SteamId}.jpg";
            }
        }

        /// <summary>
        /// Checks ban status for all AppData.SteamUsers
        /// Then updates with ban info
        /// Saves VAC info into SteamVACCache.json as well for caching
        /// </summary>
        private static void WebApiGetBans()
        {
            // Web API can take up to 100 items at once.
            for (var i = 0; i < AppData.SteamUsers.Count; i += 100)
            {
                var steamUsersGroup = AppData.SteamUsers.Skip(i).Take(100);
                var steamIds = steamUsersGroup.Select(su => su.SteamId).ToList();
                var uri =
                    $"https://api.steampowered.com/ISteamUser/GetPlayerBans/v0001/?key={SteamSettings.SteamWebApiKey}&steamids={string.Join(',', steamIds)}";
                try
                {
                    Globals.ReadWebUrl(uri, out var jsonString);
                    if (!IsSteamApiKeyValid(jsonString)) return;
                    var json = JObject.Parse(jsonString);

                    if (!json!.ContainsKey("players")) return;
                    var players = json.Value<JArray>("players");

                    foreach (var player in players!)
                    {
                        var steamId = player["SteamId"]!.Value<string>();

                        // Update Vac and Limited in AppData.SteamUsers
                        var su = AppData.SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
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
        private static bool IsSteamApiKeyValid(string jsResponse)
        {
            if (jsResponse.StartsWith("<html>"))
            {
                // Error: Key was likely invalid.
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_SteamWebKeyInvalid"],
                    renderTo: "toastarea");

                SteamSettings.SteamWebApiKey = "";
                SteamSettings.SaveSettings();
                SteamSettings.SteamWebApiWasReset = true;
                return false;
            }

            return true;
        }

        /// <summary>
        /// Checks for SteamVACCache.json, and assuming all info is there: Updates AppSettings.SteamUsers.
        /// Otherwise it updates all VAC/Limited info from Web API.
        /// </summary>
        private static void WebApiPrepareBans()
        {
            _ = GeneralFuncs.DeletedOutdatedFile(SteamSettings.VacCacheFile, SteamSettings.ImageExpiryTime);
            if (File.Exists(SteamSettings.VacCacheFile))
            {
                var vsList = JsonConvert.DeserializeObject<List<VacStatus>>(Globals.ReadAllText(SteamSettings.VacCacheFile));
                if (vsList != null && vsList.Count == AppData.SteamUsers.Count)
                {
                    foreach (var vs in vsList)
                    {
                        var su = AppData.SteamUsers.FirstOrDefault(x => x.SteamId == vs.SteamId);
                        if (su == null) continue;

                        su.Vac = vs.Vac;
                        su.Limited = vs.Ltd;
                    }

                    return;
                }
            }

            // File doesn't exist, has an error or has a different number of users > Refresh all vac info.
            WebApiGetBans();
        }



        /// <summary>
        /// Restart Steam with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="steamId">(Optional) User's SteamID</param>
        /// <param name="ePersonaState">(Optional) Persona state for user [0: Offline, 1: Online...]</param>
        /// <param name="args">Starting arguments</param>
        public static void SwapSteamAccounts(string steamId = "", int ePersonaState = -1, string args = "")
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SwapSteamAccounts] Swapping to: hidden. ePersonaState={ePersonaState}");
            if (steamId != "" && !VerifySteamId(steamId))
            {
                return;
            }

            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatform", new { platform  = "Steam" }]);
            if (!GeneralFuncs.CloseProcesses(SteamSettings.Processes, SteamSettings.ClosingMethod))
            {
                if (Globals.IsAdministrator)
                    _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = "Steam" }]);
                else
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                    _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
                }
                return;
            }

            if (OperatingSystem.IsWindows()) UpdateLoginUsers(steamId, ePersonaState);

            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_StartingPlatform", new { platform = "Steam" }]);
            if (SteamSettings.AutoStart)
            {
                if (SteamSettings.StartSilent) args += " -silent";
                if (SteamSettings.OldUi) args += " -vgui";

                _ = Globals.StartProgram(SteamSettings.Exe(), SteamSettings.Admin, args, SteamSettings.StartingMethod)
                    ? GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new {platform = "Steam"}], renderTo: "toastarea")
                    : GeneralInvocableFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new {platform = "Steam"}], renderTo: "toastarea");
            }

            if (SteamSettings.AutoStart && AppSettings.MinimizeOnSwitch) _ = AppData.InvokeVoidAsync("hideWindow");

            NativeFuncs.RefreshTrayArea();
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
            AppStats.IncrementSwitches("Steam");

            try
            {
                SteamSettings.LastAccName = steamId;
                SteamSettings.LastAccTimestamp = Globals.GetUnixTimeInt();
                if (SteamSettings.LastAccName != "") _ = AppData.InvokeVoidAsync("highlightCurrentAccount", SteamSettings.LastAccName);
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
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.VerifySteamId] Verifying SteamID: {steamId.Substring(steamId.Length - 4, 4)}");
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
        public static void UpdateLoginUsers(string selectedSteamId, int pS)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.UpdateLoginUsers] Updating loginusers: selectedSteamId={(selectedSteamId.Length > 0 ? selectedSteamId.Substring(selectedSteamId.Length - 4, 4) : "")}, pS={pS}");
            var userAccounts = GetSteamUsers(SteamSettings.LoginUsersVdf());
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_UpdatingFile", new { file = "loginusers.vdf" }]);
            var tempFile = SteamSettings.LoginUsersVdf() + "_temp";
            Globals.DeleteFile(tempFile);

            // MostRec is "00" by default, just update the one that matches SteamID.
            try
            {
                userAccounts.Where(x => x.SteamId == selectedSteamId).ToList().ForEach(u =>
                {
                    u.MostRec = "1";
                    u.RememberPass = "1";
                    u.OfflineMode = pS == -1 ? u.OfflineMode : (pS > 1 ? "0" : pS == 1 ? "0" : "1");
                    u.SkipOfflineModeWarning = pS == -1 ? u.OfflineMode : (pS > 1 ? "0" : pS == 1 ? "0" : "1");
                    // u.OfflineMode: Set ONLY if defined above
                    // If defined & > 1, it's custom, therefor: Online
                    // Otherwise, invert [0 == Offline => Online, 1 == Online => Offline]
                });
            }
            catch (InvalidOperationException)
            {
                GeneralInvocableFuncs.ShowToast("error", Lang["Toast_MissingUserId"]);
            }
            //userAccounts.Single(x => x.SteamId == selectedSteamId).MostRec = "1";

            // Save updated loginusers.vdf
            SaveSteamUsersIntoVdf(userAccounts);

            // -----------------------------------
            // - Update localconfig.vdf for user -
            // -----------------------------------
            if (pS != -1) SetPersonaState(selectedSteamId, pS); // Update persona state, if defined above.

            // -----------------------------------
            // - Update config.vdf for new Steam Switcher
            // -----------------------------------
            try
            {
                SetShowSteamSwitcher();
            }
            catch (Exception)
            {
                GeneralInvocableFuncs.ShowToast("error", Lang["CouldntModifyX", new { file = Path.Join(SteamSettings.FolderPath, "config", "config.vdf") }]);
            }

            Index.Steamuser user = new() { AccName = "" };
            try
            {
                if (selectedSteamId != "")
                    user = userAccounts.Single(x => x.SteamId == selectedSteamId);
            }
            catch (InvalidOperationException)
            {
                GeneralInvocableFuncs.ShowToast("error", Lang["Toast_MissingUserId"]);
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
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_UpdatingRegistry"]);
            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam");
            key?.SetValue("AutoLoginUser", user.AccName); // Account name is not set when changing user accounts from launch arguments (part of the viewmodel). -- Can be "" if no account
            key?.SetValue("RememberPassword", 1);

            // -----------------------------------
            // ------Update Tray users list ------
            // -----------------------------------
            if (selectedSteamId != "")
                Globals.AddTrayUser("Steam", "+s:" + user.SteamId, SteamSettings.TrayAccName ? user.AccName : GetName(user), SteamSettings.TrayAccNumber);
        }

        /// <summary>
        /// Save updated list of Steamuser into loginusers.vdf, in vdf format.
        /// </summary>
        /// <param name="userAccounts">List of Steamuser to save into loginusers.vdf</param>
        public static void SaveSteamUsersIntoVdf(List<Index.Steamuser> userAccounts)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SaveSteamUsersIntoVdf] Saving updated loginusers.vdf. Count: {userAccounts.Count}");
            // Convert list to JObject list, ready to save into vdf.
            var outJObject = new JObject();
            foreach (var ua in userAccounts)
            {
                outJObject[ua.SteamId] = (JObject)JToken.FromObject(ua);
            }

            // Write changes to files.
            var tempFile = SteamSettings.LoginUsersVdf() + "_temp";
            File.WriteAllText(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            if (!File.Exists(tempFile))
            {
                File.Replace(tempFile, SteamSettings.LoginUsersVdf(), SteamSettings.LoginUsersVdf() + "_last");
                return;
            }
            try
            {
                // Let's try break this down, as some users are having issues with the above function.
                // Step 1: Backup
                if (File.Exists(SteamSettings.LoginUsersVdf()))
                {
                    File.Copy(SteamSettings.LoginUsersVdf(), SteamSettings.LoginUsersVdf() + "_last", true);
                }

                // Step 2: Write new info
                File.WriteAllText(SteamSettings.LoginUsersVdf(), @"""users""" + Environment.NewLine + outJObject.ToVdf());
            }
            catch (Exception ex)
            {
                Globals.WriteToLog("Failed to swap Steam users! Could not create temp loginusers.vdf file, and replace original using workaround! Contact TroubleChute.", ex);
                GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotFindX", new { x = tempFile }]);
            }
        }

        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        /// <returns>Whether files were deleted or not</returns>
        public static void ClearImages()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.ClearImages] Clearing images.");
            if (!Directory.Exists(SteamSettings.SteamImagePath))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_CantClearImages"], Lang["Error"], "toastarea");
            }
            Globals.DeleteFiles(SteamSettings.SteamImagePath);
            // Reload page, then display notification using a new thread.
            AppData.ActiveNavMan?.NavigateTo(
                $"/steam/?cacheReload&toast_type=success&toast_title={Uri.EscapeDataString(Lang["Success"])}&toast_message={Uri.EscapeDataString(Lang["Toast_ClearedImages"])}", true);
        }

        /// <summary>
        /// Sets whether the user is invisible or not
        /// </summary>
        /// <param name="steamId">SteamID of user to update</param>
        /// <param name="ePersonaState">Persona state enum for user (0-7)</param>
        public static void SetPersonaState(string steamId, int ePersonaState)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SetPersonaState] Setting persona state for: {steamId.Substring(steamId.Length - 4, 4)}, To: {ePersonaState}");
            // Values:
            // 0: Offline, 1: Online, 2: Busy, 3: Away, 4: Snooze, 5: Looking to Trade, 6: Looking to Play, 7: Invisible
            var id32 = new SteamIdConvert(steamId).Id32; // Get SteamID
            var localConfigFilePath = Path.Join(SteamSettings.FolderPath, "userdata", id32, "config", "localconfig.vdf");
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

        public static void SetShowSteamSwitcher()
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SetShowSteamSwitcher] Setting ShowSteamSwitcher to: {SteamSettings.ShowSteamSwitcher}");
            var localConfigFilePath = Path.Join(SteamSettings.FolderPath, "config", "config.vdf");
            if (!File.Exists(localConfigFilePath)) return;
            var localConfigText = Globals.ReadAllText(localConfigFilePath); // Read relevant config.vdf

            // Replace entire line that contains "AlwaysShowUserChooser"
            var lines = localConfigText.Split('\n');
            for (var i = 0; i < lines.Length; i++)
            {
                if (lines[i].Contains("AlwaysShowUserChooser"))
                {
                    lines[i] = $"\"AlwaysShowUserChooser\" \"{(SteamSettings.ShowSteamSwitcher ? "1" : "0")}\"";
                    break;
                }
            }

            // Output
            File.WriteAllText(localConfigFilePath, string.Join('\n', lines));
        }

        /// <summary>
        /// Returns string representation of Steam ePersonaState int
        /// </summary>
        /// <param name="ePersonaState">integer state to return string for</param>
        public static string PersonaStateToString(int ePersonaState)
        {
            return ePersonaState switch
            {
                -1 => "",
                0 => Lang["Offline"],
                1 => Lang["Online"],
                2 => Lang["Busy"],
                3 => Lang["Away"],
                4 => Lang["Snooze"],
                5 => Lang["LookingToTrade"],
                6 => Lang["LookingToPlay"],
                7 => Lang["Invisible"],
                _ => Lang["Unrecognized_EPersonaState"]
            };
        }
        #endregion

        #region STEAM_SETTINGS
        /* OTHER FUNCTIONS*/
        // STEAM SPECIFIC -- Move to a new file in the future.

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetSteamForgetAcc() => Task.FromResult(SteamSettings.ForgetAccountEnabled);

        [JSInvokable]
        public static Task<string> GetSteamNotes(string id) => Task.FromResult(SteamSettings.AccountNotes.GetValueOrDefault(id) ?? "");

        [JSInvokable]
        public static void SetSteamNotes(string id, string note)
        {
            SteamSettings.AccountNotes[id] = note;
            SteamSettings.SaveSettings();
        }


        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="steamId">SteamId of account to be removed</param>
        public static bool ForgetAccount(string steamId)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.ForgetAccount] Forgetting account: {steamId.Substring(steamId.Length - 4, 4)}");
            // Load and remove account that matches SteamID above.
            var userAccounts = GetSteamUsers(SteamSettings.LoginUsersVdf());
            _ = userAccounts.RemoveAll(x => x.SteamId == steamId);

            // Save updated loginusers.vdf file
            SaveSteamUsersIntoVdf(userAccounts);

            // Remove image
            var img = $"{SteamSettings.SteamImagePathHtml}{steamId}.jpg";
            Globals.DeleteFile(img);

            // Remove from Tray
            Globals.RemoveTrayUserByArg("Steam", "+s:" + steamId);
            return true;
        }
        #endregion
    }
}
