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
using System.IO;
using System.Linq;
using System.Net;
using System.Threading.Tasks;
using System.Xml;
using Gameloop.Vdf;
using Gameloop.Vdf.JsonConverter;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Classes.Steam;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class SteamState : ISteamState
{
    private readonly IAppState _appState;
    private readonly IGameStats _gameStats;
    private readonly ILang _lang;
    private readonly IModals _modals;
    private readonly ISharedFunctions _sharedFunctions;
    private readonly IStatistics _statistics;
    private readonly ISteamSettings _steamSettings;
    private readonly IToasts _toasts;
    private readonly ITemplatedPlatformState _templatedPlatformState;

    public List<SteamUser> SteamUsers { get; set; } = new();
    public bool SteamLoadingProfiles { get; set; }
    public SteamContextMenu ContextMenu { get; set; } = new();

    public bool DesktopShortcut
    {
        get => Shortcut.CheckShortcuts("Steam");
        set => Shortcut.PlatformDesktopShortcut(Environment.GetFolderPath(Environment.SpecialFolder.Desktop),
            "Steam", "s", value);
    }

    public bool SteamWebApiWasReset { get; set; }
    private const string ShortcutFolder = "LoginCache\\Steam\\Shortcuts\\";
    private static readonly string[] ShortcutFolders = new[] { "%StartMenuAppData%\\Steam\\" };

    #region LOADING

    /// <summary>
    /// Main function for Steam Account Switcher. Run on load.
    /// Collects accounts from Steam's loginusers.vdf
    /// Prepares images and VAC/Limited status
    /// Prepares HTML Elements string for insertion into the account switcher GUI.
    /// </summary>
    public SteamState(IAppState appState, IGameStats gameStats, ILang lang, ISteamSettings steamSettings,
        IModals modals, IStatistics statistics, ISharedFunctions sharedFunctions, IToasts toasts,
        ITemplatedPlatformState templatedPlatformState)
    {
        _toasts = toasts;
        _appState = appState;
        _lang = lang;
        _steamSettings = steamSettings;
        _modals = modals;
        _statistics = statistics;
        _sharedFunctions = sharedFunctions;
        _gameStats = gameStats;
        _templatedPlatformState = templatedPlatformState;
    }

    public async Task LoadSteamState(ISteamFuncs steamFuncs, IJSRuntime jsRuntime)
    {

        if (SteamLoadingProfiles) return;
        Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.LoadProfiles] Loading Steam profiles");
        SteamLoadingProfiles = true;

        SteamUsers = GetSteamUsers(_steamSettings.LoginUsersVdf);

        // Order
        if (File.Exists("LoginCache\\Steam\\order.json"))
        {
            var savedOrder =
                JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\Steam\\order.json"));
            if (savedOrder != null)
            {
                var index = 0;
                if (savedOrder is { Count: > 0 })
                    foreach (var acc in from i in savedOrder
                                        where SteamUsers.Any(x => x.SteamId == i)
                                        select SteamUsers.Single(x => x.SteamId == i))
                    {
                        SteamUsers.Remove(acc);
                        SteamUsers.Insert(Math.Min(index, SteamUsers.Count), acc);
                        index++;
                    }
            }
        }

        // If Steam Web API key to be used instead
        if (_steamSettings.SteamWebApiKey != "")
        {
            // Handle all image downloads
            await WebApiPrepareImages();
            WebApiPrepareBans();

            // Key was fine? Continue. If not, the non-api method will be used.
            if (!SteamWebApiWasReset)
            {
                _appState.Switcher.SteamAccounts.Clear();
                foreach (var su in SteamUsers)
                {
                    InsertAccount(su);
                }
            }
        }

        // If Steam Web API key was not used, or was reset (encountered an error)
        if (_steamSettings.SteamWebApiKey == "" || SteamWebApiWasReset)
        {
            SteamWebApiWasReset = false;

            // Load cached ban info
            LoadCachedBanInfo();
            _appState.Switcher.SteamAccounts.Clear();

            foreach (var su in SteamUsers)
            {
                // If not cached: Get ban status and images
                // If cached: Just get images
                PrepareProfile(su.SteamId,
                    !su.BanInfoLoaded); // Get ban status as well if was not loaded (missing from cache)

                InsertAccount(su);
            }

            SaveVacInfo();
            LoadBasicCompat();
        }

        // Load notes
        LoadNotes();

        await _sharedFunctions.FinaliseAccountList(jsRuntime);

        _statistics.SetAccountCount("Steam", SteamUsers.Count);
        SteamLoadingProfiles = false;

        ContextMenu = new SteamContextMenu(jsRuntime, _appState, _gameStats, _lang, _modals, _sharedFunctions, steamFuncs, _steamSettings, this, _toasts);
    }


    /// <summary>
    /// Installed games on this computer
    /// </summary>
    public List<string> InstalledGames { get; set; }
    /// <summary>
    /// List of AppIds and AppNames on this computer
    /// </summary>
    public Dictionary<string, string> AppIds { get; set; }

    public void SaveAccountOrder(string jsonString)
    {
        var file = "LoginCache\\Steam\\order.json";
        // Create folder if it doesn't exist:
        var folder = Path.GetDirectoryName(file);
        if (folder != "") _ = Directory.CreateDirectory(folder ?? string.Empty);
        File.WriteAllText(file, jsonString);
    }

    public string GetName(SteamUser su) => string.IsNullOrWhiteSpace(su.Name) ? su.AccName : su.Name;
    private void InsertAccount(SteamUser su)
    {
        var account = new Account
        {
            Platform = "Steam",
            AccountId = su.SteamId,
            DisplayName = GeneralFuncs.EscapeText(GetName(su)),
            Classes = new Account.ClassCollection()
            {
                Image = (_steamSettings.ShowVac && su.Vac ? " status_vac" : "") + (_steamSettings.ShowLimited && su.Limited ? " status_limited" : ""),
                Line0 = "streamerCensor",
                Line2 = "streamerCensor steamId",
                Line3 = "lastLogin"
            },
            LoginUsername = su.AccName,
            UserStats = _gameStats.GetUserStatsAllGamesMarkup(su.SteamId),
            ImagePath = su.ImgUrl,
            Line0 = _steamSettings.ShowAccUsername ? su.AccName : "",
            Line2 = su.SteamId,
            Line3 = Globals.UnixTimeStampToDateTime(su.LastLogin)
        };
        _appState.Switcher.SteamAccounts.Add(account);
    }


    /// <summary>
    /// Loads ban info from cache file
    /// </summary>
    /// <returns>Whether file was loaded. False if deleted ~ failed to load.</returns>
    public void LoadCachedBanInfo()
    {
        Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.LoadCachedBanInfo]");
        _ = GeneralFuncs.DeletedOutdatedFile(_steamSettings.VacCacheFile, _steamSettings.ImageExpiryTime);
        if (!File.Exists(_steamSettings.VacCacheFile)) return;

        // Load list of banStatus
        var banInfoList = JsonConvert.DeserializeObject<List<VacStatus>>(Globals.ReadAllText(_steamSettings.VacCacheFile));

        if (banInfoList == null) return;
        foreach (var su in SteamUsers)
        {
            var banInfo = banInfoList.FirstOrDefault(x => x.SteamId == su.SteamId);
            su.BanInfoLoaded = banInfo != null;
            if (!su.BanInfoLoaded) continue;

            su.Vac = banInfo!.Vac;
            su.Limited = banInfo.Ltd;
        }
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
        var su = SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
        if (su == null) return;
        Globals.DebugWriteLine(
            $@"[Func:Steam\SteamSwitcherFuncs.PrepareProfile] Preparing image and ban info for: {su.SteamId.Substring(su.SteamId.Length - 4, 4)}");
        _ = Directory.CreateDirectory(_steamSettings.SteamImagePath);

        var dlDir = $"{_steamSettings.SteamImagePath}{su.SteamId}.jpg";
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
            _ = GeneralFuncs.DeletedOutdatedFile(dlDir, _steamSettings.ImageExpiryTime);
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
                su.ImgUrl = $"{_steamSettings.SteamImagePathHtml}{su.SteamId}.jpg";
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
            ? $"{_steamSettings.SteamImagePathHtml}{su.SteamId}.jpg"
            : "img/QuestionMark.jpg";
    }

    /// <summary>
    /// Gets ban status and image from input XML Document.
    /// </summary>
    /// <param name="profileXml">User's profile XML string</param>
    private void ProcessSteamUserXml(XmlDocument profileXml)
    {
        Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.XmlGetVacLimitedStatus] Get Ban status for Steam account.");

        // Read SteamID and select the correct Steam User to edit
        var steamId = profileXml.DocumentElement?.SelectNodes("/profile/steamID64")?[0]?.InnerText;
        if (steamId == null) return;
        var su = SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
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
    /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
    /// </summary>
    /// <returns>(Steam's path)\config\loginusers.vdf</returns>
    public string CheckLoginUsersVdfPath()
    {
        Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.LoginUsersVdf]");
        if (File.Exists(_steamSettings.LoginUsersVdf)) return _steamSettings.LoginUsersVdf;

        _steamSettings.FolderPath = "";
        _steamSettings.Save();
        return "RESET_PATH";
    }

    public bool SteamSettingsValid() => CheckLoginUsersVdfPath() != "RESET_PATH";

    /// <summary>
    /// Takes loginusers.vdf and iterates through each account, loading details into output Steamuser list.
    /// </summary>
    /// <param name="loginUserPath">loginusers.vdf path</param>
    /// <returns>List of SteamUser classes, from loginusers.vdf</returns>
    public List<SteamUser> GetSteamUsers(string loginUserPath)
    {
        Globals.DebugWriteLine(
            $@"[Func:Steam\SteamSwitcherFuncs.GetSteamUsers] Getting list of Steam users from {loginUserPath}");
        _ = Directory.CreateDirectory("wwwroot/img/profiles");

        if (LoadFromVdf(loginUserPath, out var userAccounts)) return userAccounts;

        // Didn't work, try last file, if exists.
        var lastVdf = loginUserPath.Replace(".vdf", ".vdf_last");
        if (!File.Exists(lastVdf) || !LoadFromVdf(lastVdf, out userAccounts)) return new List<SteamUser>();

        _toasts.ShowToastLang(ToastType.Info, "Toast_PartiallyFixed", "Toast_Steam_VdfLast", 10000);
        return userAccounts;
    }


    private class LoginUsersFile
    {
        public Dictionary<string, LoginUserAccount> Users { get; set; }
    }
    private class LoginUserAccount
    {
        public string AccountName { get; set; } = "";
        public string PersonaName { get; set; } = "";
        public string RememberPassword { get; set; } = "";
        [JsonProperty("mostrecent")] public string MostRecent { get; set; } = "";
        public string Timestamp { get; set; } = "";
        public string WantsOfflineMode { get; set; } = "";
    }


    private bool LoadFromVdf(string vdf, out List<SteamUser> userAccounts)
    {
        userAccounts = new List<SteamUser>();

        try
        {
            var vdfText = VerifyVdfText(vdf);

            var loginUsersVToken = VdfConvert.Deserialize(vdfText);
            var jLoginUsers = new JObject {loginUsersVToken.ToJson()};

            var loginUsers = jLoginUsers.ToObject<LoginUsersFile>();
            userAccounts.AddRange(from u in loginUsers?.Users
                let details = u.Value
                select new SteamUser
                {
                    SteamId = u.Key,
                    Name = details.PersonaName,
                    AccName = details.AccountName,
                    MostRec = details.MostRecent,
                    LastLogin = details.Timestamp,
                    OfflineMode = details.WantsOfflineMode,
                });
        }
        catch (FileNotFoundException)
        {
            _toasts.ShowToastLang(ToastType.Error, "NotFound", "Toast_Steam_FailedLoginusers");
            return false;
        }
        catch (AggregateException)
        {
            _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_Steam_FailedLoginusers");
            return false;
        }
        catch (Exception)
        {
            _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_Steam_FailedLoginusers");
            return false;
        }

        return true;
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
    /// Checks for SteamVACCache.json, and assuming all info is there: Updates WindowSettings.SteamUsers.
    /// Otherwise it updates all VAC/Limited info from Web API.
    /// </summary>
    private void WebApiPrepareBans()
    {
        _ = GeneralFuncs.DeletedOutdatedFile(_steamSettings.VacCacheFile, _steamSettings.ImageExpiryTime);
        if (File.Exists(_steamSettings.VacCacheFile))
        {
            var vsList = JsonConvert.DeserializeObject<List<VacStatus>>(Globals.ReadAllText(_steamSettings.VacCacheFile));
            if (vsList != null && vsList.Count == SteamUsers.Count)
            {
                foreach (var vs in vsList)
                {
                    var su = SteamUsers.FirstOrDefault(x => x.SteamId == vs.SteamId);
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
    /// Download all missing or outdated Steam profile images, multi-threaded
    /// </summary>
    private async Task WebApiPrepareImages()
    {
        // Create a queue of SteamUsers to download images for
        var queue = new List<SteamUser>();

        foreach (var su in SteamUsers)
        {
            var dlDir = $"{_steamSettings.SteamImagePath}{su.SteamId}.jpg";
            // Delete outdated file, if it exists
            var wasDeleted = GeneralFuncs.DeletedOutdatedFile(dlDir, _steamSettings.ImageExpiryTime);
            // ... & invalid files
            wasDeleted = wasDeleted || GeneralFuncs.DeletedInvalidImage(dlDir);

            if (wasDeleted) queue.Add(su);
        }

        // Either return if queue is empty, or download queued images and then continue.
        if (queue.Count != 0)
        {
            await _toasts.ShowToastLangAsync(ToastType.Info, "Toast_DownloadingProfileData");
            await Globals.MultiThreadParallelDownloads(WebApiGetImageList(queue)).ConfigureAwait(true);
        }

        // Set the correct path
        foreach (var su in SteamUsers)
        {
            su.ImgUrl = $"{_steamSettings.SteamImagePathHtml}{su.SteamId}.jpg";
        }
    }

    /// <summary>
    /// Checks ban status for all AppData.SteamUsers
    /// Then updates with ban info
    /// Saves VAC info into SteamVACCache.json as well for caching
    /// </summary>
    private void WebApiGetBans()
    {
        // Web API can take up to 100 items at once.
        for (var i = 0; i < SteamUsers.Count; i += 100)
        {
            var steamUsersGroup = SteamUsers.Skip(i).Take(100);
            var steamIds = steamUsersGroup.Select(su => su.SteamId).ToList();
            var uri =
                $"https://api.steampowered.com/ISteamUser/GetPlayerBans/v0001/?key={_steamSettings.SteamWebApiKey}&steamids={string.Join(',', steamIds)}";
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
                    var su = SteamUsers.FirstOrDefault(x => x.SteamId == steamId);
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
    private bool IsSteamApiKeyValid(string jsResponse)
    {
        if (!jsResponse.StartsWith("<html>")) return true;

        // Error: Key was likely invalid.
        _toasts.ShowToastLang(ToastType.Error, "Toast_SteamWebKeyInvalid");

        _steamSettings.SteamWebApiKey = "";
        _steamSettings.Save();
        SteamWebApiWasReset = true;
        return false;
    }

    /// <summary>
    /// Saves List of VacStatus into cache file as JSON.
    /// </summary>
    public void SaveVacInfo()
    {
        var vsList = SteamUsers.Select(su => new VacStatus
            {
                SteamId = su.SteamId,
                Vac = su.Vac,
                Ltd = su.Limited
            })
            .ToList();

        _ = Directory.CreateDirectory(Path.GetDirectoryName(_steamSettings.VacCacheFile) ?? string.Empty);
        File.WriteAllText(_steamSettings.VacCacheFile, JsonConvert.SerializeObject(vsList));
    }

    /// <summary>
    /// Gets a list of profile image URLs for all profiles
    /// This replaces GetUserImageUrl, and does it for every SteamId at once
    /// </summary>
    /// <returns>Dictionary of [Profile image URL] = Destination file path</returns>
    private Dictionary<string, string> WebApiGetImageList(IReadOnlyCollection<SteamUser> steamUsers)
    {
        var images = new Dictionary<string, string>();
        // Web API can take up to 100 items at once.
        for (var i = 0; i < steamUsers.Count; i += 100)
        {
            var steamUsersGroup = steamUsers.Skip(i).Take(100);
            var steamIds = steamUsersGroup.Select(su => su.SteamId).ToList();
            var uri =
                $"https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key={_steamSettings.SteamWebApiKey}&steamids={string.Join(',', steamIds)}";
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
                if (steamId != null && imageUrl != null) images.Add(imageUrl, $"{_steamSettings.SteamImagePath}/{steamId}.jpg");
            }
        }

        return images;
    }

    #endregion

    #region Basic Compatability
    private void LoadBasicCompat()
    {
        if (OperatingSystem.IsWindows())
        {
            var shortcutImagePath = Path.Join(Globals.UserDataFolder, "wwwroot\\img\\shortcuts\\Steam\\");

            // Add image for main platform button:
            var imagePath = Path.Join(shortcutImagePath, "Steam.png");

            if (!File.Exists(imagePath))
            {
                // Search start menu for Steam.
                var startMenuFiles = Directory.GetFiles(Globals.ExpandEnvironmentVariables("%StartMenuAppData%"), "Steam.lnk", SearchOption.AllDirectories);
                var commonStartMenuFiles = Directory.GetFiles(Globals.ExpandEnvironmentVariables("%StartMenuProgramData%"), "Steam.lnk", SearchOption.AllDirectories);
                if (startMenuFiles.Length > 0)
                    Globals.SaveIconFromFile(startMenuFiles[0], imagePath);
                else if (commonStartMenuFiles.Length > 0)
                    Globals.SaveIconFromFile(commonStartMenuFiles[0], imagePath);
                else
                    Globals.SaveIconFromFile(_steamSettings.Exe, imagePath);
            }


            foreach (var sFolder in ShortcutFolders)
            {
                if (sFolder == "") continue;
                // Foreach file in folder
                var desktopShortcutFolder = Globals.ExpandEnvironmentVariables(sFolder);
                if (!Directory.Exists(desktopShortcutFolder)) continue;
                foreach (var shortcut in new DirectoryInfo(desktopShortcutFolder).GetFiles())
                {
                    var fName = shortcut.Name;
                    if (Globals.RemoveShortcutExt(fName) == "Steam") continue; // Ignore self

                    // Check if in saved shortcuts and If ignored
                    if (File.Exists(Path.Join(ShortcutFolder, fName.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"))))
                    {
                        imagePath = Path.Join(shortcutImagePath, Globals.RemoveShortcutExt(fName) + ".png");
                        if (File.Exists(imagePath)) File.Delete(imagePath);
                        fName = fName.Replace("_ignored", "");
                        if (_steamSettings.Shortcuts.ContainsValue(fName))
                            _steamSettings.Shortcuts.Remove(_steamSettings.Shortcuts.First(e => e.Value == fName).Key);
                        continue;
                    }

                    Directory.CreateDirectory(ShortcutFolder);
                    var outputShortcut = Path.Join(ShortcutFolder, fName);

                    // Exists and is not ignored: Update shortcut
                    Globals.CopyFile(shortcut.FullName, outputShortcut);
                    // Organization will be saved in HTML/JS
                }
            }

            // Now get images for all the shortcuts in the folder, as long as they don't already exist:
            List<string> existingShortcuts = new();
            if (Directory.Exists(ShortcutFolder))
                foreach (var f in new DirectoryInfo(ShortcutFolder).GetFiles())
                {
                    if (f.Name.Contains("_ignored")) continue;
                    var imageName = Globals.RemoveShortcutExt(f.Name) + ".png";
                    imagePath = Path.Join(shortcutImagePath, imageName);
                    existingShortcuts.Add(f.Name);
                    if (!_steamSettings.Shortcuts.ContainsValue(f.Name))
                    {
                        // Not found in list, so add!
                        var last = 0;
                        foreach (var (k, _) in _steamSettings.Shortcuts)
                            if (k > last) last = k;
                        last += 1;
                        _steamSettings.Shortcuts.Add(last, f.Name); // Organization added later
                    }

                    // Extract image and place in wwwroot (Only if not already there):
                    if (!File.Exists(imagePath))
                    {
                        Globals.SaveIconFromFile(f.FullName, imagePath);
                    }
                }

            foreach (var (i, s) in _steamSettings.Shortcuts)
            {
                if (!existingShortcuts.Contains(s))
                    _steamSettings.Shortcuts.Remove(i);
            }
        }

        _statistics.SetGameShortcutCount("Steam", _steamSettings.Shortcuts);
        _steamSettings.Save();
    }

    public void RunSteam(bool admin, string args)
    {
        if (Globals.StartProgram(_steamSettings.Exe, admin, args, _steamSettings.StartingMethod))
            _toasts.ShowToastLang(ToastType.Info, new LangSub("Status_StartingPlatform", new { platform = "Steam" }));
        else
            _toasts.ShowToastLang(ToastType.Error, new LangSub("Toast_StartingPlatformFailed", new { platform = "Steam" }));
    }
    #endregion

    #region Steam Funcs
    public void LoadNotes()
    {
        const string filePath = "LoginCache\\Steam\\AccountNotes.json";
        if (!File.Exists(filePath)) return;

        var loaded = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(filePath));
        if (loaded is null) return;

        foreach (var (key, val) in loaded)
        {
            var acc = _appState.Switcher.SteamAccounts.FirstOrDefault(x => x.AccountId == key);
            if (acc is null) return;
            acc.Note = val;
            acc.TitleText = val;
        }

        _modals.IsShown = false;
    }
    #endregion
}