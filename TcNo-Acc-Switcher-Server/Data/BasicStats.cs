﻿using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using HtmlAgilityPack;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared.Toast;

namespace TcNo_Acc_Switcher_Server.Data
{
    public sealed class BasicStats
    {
        [Inject] private AppData AData { get; set; }
        // Make this a singleton
        private static BasicStats _instance;
        private static readonly object LockObj = new();
        public static BasicStats Instance
        {
            get
            {
                lock (LockObj)
                {
                    if (_instance != null) return _instance;

                    _instance = new BasicStats();
                    _instance.BasicStatsInit();
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
        // ---------------

        private JObject _jData;
        private static JObject JData { get => Instance._jData; set => Instance._jData = value; }
        private List<string> _gamesDict = new();
        private SortedDictionary<string, List<string>> _platformGames = new();
        private Dictionary<string, GameStat> _gameStats = new();

        /// <summary>
        /// List of all games with stats
        /// </summary>
        public static List<string> GamesDict { get => Instance._gamesDict; set => Instance._gamesDict = value; }
        /// <summary>
        /// Dictionary of Platform:List of supported game names
        /// </summary>
        public static SortedDictionary<string, List<string>> PlatformGames { get => Instance._platformGames; set => Instance._platformGames = value; }
        /// <summary>
        /// Dictionary of Game Name:Available user game statistics
        /// </summary>
        public static Dictionary<string, GameStat> GameStats { get => Instance._gameStats; set => Instance._gameStats = value; }

        public static UserGameStat GetUserGameStat(string game, string accountId) =>
            GameStats[game].CachedStats.ContainsKey(accountId) ? GameStats[game].CachedStats[accountId] : null;

        /// <summary>
        /// Get list of games for current platform - That have at least 1 account for settings.
        /// </summary>
        public static List<string> GetAllCurrentlyEnabledGames()
        {
            var games = new HashSet<string>();
            foreach (var gameStat in GameStats)
            {
                // Foreach account and game pair in GameStats
                // Add game name to list
                if (gameStat.Value.CachedStats.Any()) games.Add(gameStat.Value.Game);
            }

            return games.ToList();
        }

        /// <summary>
        /// Get and return icon html markup for specific game
        /// </summary>
        public static string GetIcon(string game, string statName) => GameStats[game].ToCollect[statName].Icon;

        public class StatValueAndIcon
        {
            public string StatValue { get; set; }
            public string IndicatorMarkup { get; set; }
        }

        // List of possible games on X platform
        public static List<string> GetAvailableGames => PlatformGames.ContainsKey(AppData.CurrentSwitcher) ? PlatformGames[AppData.CurrentSwitcher] : new List<string>();

        /// <summary>
        /// Returns list Dictionary of Game Names:[Dictionary of statistic names and StatValueAndIcon (values and indicator text for HTML)]
        /// </summary>
        public static Dictionary<string, Dictionary<string, StatValueAndIcon>> GetUserStatsAllGamesMarkup(string account = "")
        {
            if (account == "") account = AppData.SelectedAccountId;

            var returnDict = new Dictionary<string, Dictionary<string, StatValueAndIcon>>( );
            // Foreach available game
            foreach (var availableGame in GetAvailableGames)
            {
                if (!GameStats.ContainsKey(availableGame)) continue;

                // That has the requested account
                if (!GameStats[availableGame].CachedStats.ContainsKey(account)) continue;
                var gameIndicator = GameStats[availableGame].Indicator;
                //var gameUniqueId = GameStats[availableGame].UniqueId;

                var statValueIconDict = new Dictionary<string, StatValueAndIcon>();

                // Add icon or identifier to stat pair for displaying
                foreach (var (statName, statValue) in GameStats[availableGame].CachedStats[account].Collected)
                {
                    if (GameStats[availableGame].CachedStats[account].HiddenMetrics.Contains(statName)) continue;

                    // Foreach stat
                    // Check if has icon, otherwise use just indicator string
                    var indicatorMarkup = GetIcon(availableGame, statName);
                    if (string.IsNullOrEmpty(indicatorMarkup) && !string.IsNullOrEmpty(gameIndicator) && GameStats[availableGame].ToCollect[statName].SpecialType is not "ImageDownload")
                        indicatorMarkup = $"<sup>{gameIndicator}</sup>";

                    statValueIconDict.Add(statName, new StatValueAndIcon
                    {
                        StatValue = statValue,
                        IndicatorMarkup = indicatorMarkup
                    });
                }

                //returnDict[gameUniqueId] = statValueIconDict;
                returnDict[availableGame] = statValueIconDict;
            }

            return returnDict;
        }

        /// <summary>
        /// Gets list of all metric names to collect, as well as whether each is hidden or not, and the text to display in the UI checkbox.
        /// Example: "AP":"Arena Points"
        /// </summary>
        //public static Dictionary<string, Tuple<bool, string>> GetAllMetrics(string game)
        public static Dictionary<string, string> GetAllMetrics(string game)
        {
            //// Get hidden metrics for this game
            //var hiddenMetrics = new List<string>();
            //if (AppSettings.Instance.GloballyHiddenMetrics.ContainsKey(game))
            //    hiddenMetrics = AppSettings.Instance.GloballyHiddenMetrics[game];

            // Get list of all metrics and add to list.
            //var allMetrics = new Dictionary<string, Tuple<bool, string>>();
            var allMetrics = new Dictionary<string, string>();
            foreach (var (key, ci) in GameStats[game].ToCollect)
            {
                //allMetrics.Add(key, new Tuple<bool, string>(hiddenMetrics.Contains(key), ci.ToggleText));
                allMetrics.Add(key, ci.ToggleText);
            }

            return allMetrics;
        }

        public static bool PlatformHasAnyGames(string platform) => platform is not null && (PlatformGames.ContainsKey(platform) && PlatformGames[platform].Count > 0);
        public static JToken StatsDefinitions => (JObject)JData["StatsDefinitions"];
        public static JToken PlatformCompatibilities => (JObject)JData["PlatformCompatibilities"];
        public static JObject GetGame(string game) => (JObject) StatsDefinitions![game];
        public static bool GameExists(string game) => GamesDict.Contains(game);
        public static readonly string BasicStatsPath = Path.Join(Globals.AppDataFolder, "GameStats.json");

        /// <summary>
        /// Read BasicStats.json and collect game definitions, as well as platform-game relations.
        /// </summary>
        private void BasicStatsInit()
        {
            // Check if Platforms.json exists.
            // If it doesnt: Copy it from the programs' folder to the user data folder.
            if (!File.Exists(BasicStatsPath))
            {
                // Once again verify the file exists. If it doesn't throw an error here.
                AData.ShowToastLang(ToastType.Error, "Toast_FailedStatsLoad");
                Globals.WriteToLog("Failed to locate GameStats.json! This will cause a lot of stats to break.");
                return;
            }

            JData = GeneralFuncs.LoadSettings(BasicStatsPath);

            // Populate the List of all Games with stats
            var statsDefinitions = StatsDefinitions?.ToObject<Dictionary<string, JObject>>();
            GamesDict = statsDefinitions?.Keys.ToList();
            //foreach (var jToken in StatsDefinitions)
            //{
            //    GamesDict.Add(((JProperty)jToken).Name);
            //}

            // Populate the List of all Game-Platform relations
            PlatformGames = PlatformCompatibilities?.ToObject<SortedDictionary<string, List<string>>>();
            //foreach (var jToken in PlatformCompatibilities)
            //{
            //    var x = (JProperty)jToken;
            //    PlatformGames.Add(x.Name, x.Value.ToObject<List<string>>());
            //}

            // Foreach game:
            if (GamesDict is null) return;
            foreach (var game in GamesDict)
            {
                // If game not on global settings list, add it to the list.
                if (!AppSettings.Instance.GloballyHiddenMetrics.ContainsKey(game))
                    AppSettings.Instance.GloballyHiddenMetrics.Add(game, new Dictionary<string, bool>());

                var allMetrics = statsDefinitions?[game]["Collect"]?.ToObject<Dictionary<string, JObject>>();
                if (allMetrics is null) continue;
                foreach (var key in allMetrics.Keys)
                {
                    // Add to list if not there already.
                    if (!AppSettings.Instance.GloballyHiddenMetrics[game].ContainsKey(key))
                        AppSettings.Instance.GloballyHiddenMetrics[game].Add(key, false);

                }

                //// For each game's setting, make sure it exists if not already set.
                //foreach (var gameMetrics in GetAllMetrics(game))
                //{
                //    // Add to list if not there already.
                //    if (!AppSettings.Instance.GloballyHiddenMetrics[game].ContainsKey(gameMetrics.Key))
                //        AppSettings.Instance.GloballyHiddenMetrics[game].Add(gameMetrics.Key, false);
                //}

            }
        }

        /// <summary>
        /// Get longer game name from it's short unique ID.
        /// </summary>
        public static string GetGameNameFromId(string id) => GameStats.FirstOrDefault(x => x.Value.UniqueId.Equals(id, StringComparison.OrdinalIgnoreCase)).Key;
        /// <summary>
        /// Get short unique ID from game name.
        /// </summary>
        public static string GetGameIdFromName(string name) => GameStats.FirstOrDefault(x => x.Key.Equals(name, StringComparison.OrdinalIgnoreCase)).Value.UniqueId;

        public async Task RefreshAllAccounts(string game, string platform = "")
        {
            foreach (var id in GameStats[game].CachedStats.Keys)
            {
                await GameStats[game].LoadStatsFromWeb(id, platform);
            }

            if (game != "")
                GameStats[game].SaveStats();
        }

        public static List<string> PlatformGamesWithStats(string platform) => PlatformGames.ContainsKey(platform) ? GetAvailableGames : new List<string>();

        /// <summary>
        /// Loads games and stats for requested platform
        /// </summary>
        /// <returns>If successfully loaded platform</returns>
        public static async Task<bool> SetCurrentPlatform(string platform)
        {
            AppData.CurrentSwitcher = platform;
            if (!PlatformGames.ContainsKey(platform)) return false;

            GameStats = new Dictionary<string, GameStat>();
            // TODO: Verify this works as intended when more games are added.
            foreach (var game in GetAvailableGames)
            {
                var gs = new GameStat();
                await gs.SetGameStat(game);
                GameStats.Add(game, gs);
            }

            return true;
        }

        /// <summary>
        /// Returns a string with all the statistics available for the specified account
        /// </summary>
        public static string GetGameStatsString(string accountId, string sep, bool isBasic = false)
        {
            var outputString = "";
            // Foreach game in platform
            var oneAdded = false;
            foreach (var gs in GameStats)
            {
                // Check to see if it contains the requested accountID
                var cachedStats = gs.Value.CachedStats;
                if (!cachedStats.ContainsKey(accountId)) continue;

                if (!oneAdded) outputString += $"{gs.Key}:,";
                else
                {
                    if (isBasic)
                        outputString += $"{Environment.NewLine},{gs.Key}:,";
                    else
                        outputString += $"{Environment.NewLine},,,,,,{gs.Key}:,";
                    oneAdded = false;
                }

                // Add each stat from account to the string, starting with the game name.
                var collectedStats = cachedStats[accountId].Collected;
                foreach (var stat in collectedStats)
                {
                    if (oneAdded)
                    {
                        if (isBasic)
                            outputString += $"{Environment.NewLine},, {stat.Key}{sep}{stat.Value.Replace(sep, " ")}";
                        else
                            outputString +=
                                $"{Environment.NewLine},,,,,,, {stat.Key}{sep}{stat.Value.Replace(sep, " ")}";
                    }
                    else
                    {
                        outputString += $"{stat.Key}{sep}{stat.Value.Replace(sep, " ")}";
                        oneAdded = true;
                    }
                }
            }

            return outputString;
        }
    }

    /// <summary>
    /// A single game statistics definition, for use with others on a platform.
    /// </summary>
    public sealed class GameStat
    {
        [Inject] private AppData AData { get; set; }

        #region Properties

        private DateTime _lastLoadingNotification = DateTime.MinValue;

        public bool IsInit;
        public string Game;
        public string UniqueId = "";
        public string Indicator = "";
        public string Url = "";
        public string Cookies = "";
        /// <summary>
        ///Dictionary of Variable:Values for each account, for use in statistics web url. Such as Username="Kevin".
        /// </summary>
        public Dictionary<string, string> RequiredVars = new();
        /// <summary>
        /// Dictionary of StatName:CollectInstructions for collecting stats from web and info from user
        /// </summary>
        public Dictionary<string, CollectInstruction> ToCollect = new();

        private readonly string _cacheFileFolder = Path.Join(Globals.UserDataFolder, "StatsCache");
        private string CacheFilePath => Path.Join(_cacheFileFolder,Globals.GetCleanFilePath( $"{Game}.json"));
        /// <summary>
        /// Dictionary of AccountId:UserGameStats
        /// </summary>
        public SortedDictionary<string, UserGameStat> CachedStats = new();
        #endregion

        public class CollectInstruction
        {
            public string XPath { get; set; }
            public string Select { get; set; }
            public string DisplayAs { get; set; } = "%x%";
            public string ToggleText { get; set; } = "";

            // Optional
            public string SelectAttribute { get; set; } = ""; // If Select = "attribute", set the attribute to get here.
            public string SpecialType { get; set; } = ""; // Possible types: ImageDownload.
            public string NoDisplayIf { get; set; } = ""; // The DisplayAs text will not display if equal to the default value of this.
            public string Icon { get; set; } = ""; // Icon HTML markup
        }

        public async Task SetGameStat(string game)
        {
            if (!BasicStats.GameExists(game)) return;

            var jGame = BasicStats.GetGame(game);

            Game = game;
            UniqueId = (string)jGame["UniqueId"];
            Indicator = (string)jGame["Indicator"];
            Url = (string)jGame["Url"];
            Cookies = (string)jGame["RequestCookies"];

            RequiredVars = jGame["Vars"]?.ToObject<Dictionary<string, string>>();
            //foreach (var (k, v) in jGame["Vars"]?.ToObject<Dictionary<string, string>>()!)
            //{
            //    Vars.Add(k, v);
            //}

            ToCollect = jGame["Collect"]?.ToObject<Dictionary<string, CollectInstruction>>();

            if (ToCollect != null)
                foreach (var collectInstruction in ToCollect.Where(collectInstruction => collectInstruction.Key == "%PROFILEIMAGE%"))
                    collectInstruction.Value.ToggleText = Lang["ProfileImage_ToggleText"];

            //foreach (var (k, v) in jGame["Collect"]?.ToObject<Dictionary<string, CollectInstruction>>()!)
            //{
            //    Collect.Add(k, v);
            //}

            await LoadStats();

            IsInit = true;
        }

        /// <summary>
        /// Set up new accounts. Set game name if you want all accounts to save after setting values (Recommended).
        /// </summary>
        public async Task<bool> SetAccount(Dictionary<string, string> vars, List<string> hiddenMetrics)
        {
            if (CachedStats.ContainsKey(AppData.SelectedAccountId))
            {
                CachedStats[AppData.SelectedAccountId].Vars = vars;
                CachedStats[AppData.SelectedAccountId].HiddenMetrics = hiddenMetrics;
            }
            else
                CachedStats[AppData.SelectedAccountId] = new UserGameStat() { Vars = vars, HiddenMetrics = hiddenMetrics };

            if (CachedStats[AppData.SelectedAccountId].Collected.Count == 0 || DateTime.Now.Subtract(CachedStats[AppData.SelectedAccountId].LastUpdated).Days >= 1)
            {
                AData.ShowToastLang(ToastType.Info, "Toast_LoadingStats");
                _lastLoadingNotification = DateTime.Now;
                return await LoadStatsFromWeb(AppData.SelectedAccountId, AppData.CurrentSwitcher);
            }
            else
                SaveStats();

            return true;
        }

        /// <summary>
        /// Return URL with saved vars subbed in
        /// </summary>
        private string UrlSubbed(UserGameStat userStat)
        {
            var completeUrl = Url;
            foreach (var (key, value) in userStat.Vars)
            {
                completeUrl = completeUrl.Replace($"{{{key}}}", value);
            }
            return completeUrl;
        }

        /// <summary>
        /// Collects user statistics for account
        /// </summary>
        /// <returns>Successful or not</returns>
        public async Task<bool> LoadStatsFromWeb(string accountId, string platform = "")
        {
            // Check if init, and list of accounts contains Id
            if (!IsInit || !CachedStats.ContainsKey(accountId)) return false;
            var userStat = CachedStats[accountId];

            // Check if account has required variables set.
            if (userStat.Vars.Count == 0) return false;

            // Notify user than an account is being loaded - >5 seconds apart.
            if (DateTime.Now.Subtract(_lastLoadingNotification).Seconds >= 5)
            {
                AData.ShowToastLang(ToastType.Info, "Toast_LoadingStats");
                _lastLoadingNotification = DateTime.Now;
            }

            // Read doc from web
            HtmlDocument doc = new();
            if (!Globals.GetWebHtmlDocument(ref doc, UrlSubbed(userStat), out var responseText, Cookies))
            {
                AData.ShowToastLang(ToastType.Error, new LangSub("Toast_GameStatsLoadFail", new {Game}));
                return false;
            }

            // Collect each requested element
            foreach (var (itemName, ci) in ToCollect)
            {
                var text = "";
                HtmlNode xPathResponse;
                try
                {
                    xPathResponse = doc.DocumentNode.SelectSingleNode(ci.XPath);
                }
                catch (System.Xml.XPath.XPathException e)
                {
                    Globals.WriteToLog($"Failed to collect '{itemName}' statistic for '{Game}' ({ci.XPath})", e);
                    continue;
                }

                if (xPathResponse is null) continue;

                text = ci.Select switch
                {
                    "innerText" => xPathResponse.InnerText,
                    "innerHtml" => xPathResponse.InnerHtml,
                    "attribute" => xPathResponse.Attributes[ci.SelectAttribute]?.Value ?? "",
                    _ => text
                };

                // Don't display if equal to default
                if (text == ci.NoDisplayIf) continue;

                // Handle special items
                if (itemName == "%PROFILEIMAGE%")
                {
                    if (platform != "")
                    {
                        Globals.DownloadProfileImage(platform, accountId, text, GeneralFuncs.WwwRoot());
                    }
                    continue;
                }

                // And special Special Types
                if (ci.SpecialType == "ImageDownload")
                {
                    var fileName = $"{Indicator}-{itemName}-{accountId}.jpg";
                    var imgPath = Path.Join(GeneralFuncs.WwwRoot(), "img\\statsCache\\" + fileName);
                    if (!await Globals.DownloadFileAsync(text, imgPath))
                    {
                        continue;
                    }
                    text = "img/statsCache/" + fileName;
                }

                // Else, format as requested
                text = ci.DisplayAs.Replace("%x%", text);
                userStat.Collected[itemName] = text;
            }

            userStat.LastUpdated = DateTime.Now;

            if (!userStat.Collected.Any())
            {
                Directory.CreateDirectory(Path.Join(Globals.UserDataFolder, "temp"));
                await File.WriteAllTextAsync(Path.Join(Globals.UserDataFolder, "temp", $"download-{Globals.GetCleanFilePath(accountId)}-{Globals.GetCleanFilePath(Game)}.html"), responseText);
                AData.ShowToastLang(ToastType.Error, new LangSub("Toast_GameStatsEmpty", new { AccoundId = Globals.GetCleanFilePath(accountId), Game = Globals.GetCleanFilePath(Game) }));
                if (CachedStats.ContainsKey(accountId))
                    CachedStats.Remove(accountId);
                return false;
            }


            CachedStats[accountId] = userStat;
            SaveStats();
            return true;
        }

        /// <summary>
        /// Save all game stats for all accounts on current platform
        /// </summary>
        public void SaveStats()
        {
            _ = Directory.CreateDirectory(_cacheFileFolder);
            File.WriteAllText(CacheFilePath, JsonConvert.SerializeObject(CachedStats));
        }

        /// <summary>
        /// Load all game stats for all accounts on current platform
        /// </summary>
        public async Task LoadStats()
        {
            _ = Directory.CreateDirectory(_cacheFileFolder);

            if (!File.Exists(CacheFilePath)) return;
            var jCachedStats = GeneralFuncs.LoadSettings(CacheFilePath);
            CachedStats = jCachedStats.ToObject<SortedDictionary<string, UserGameStat>>() ?? new SortedDictionary<string, UserGameStat>();

            // Check for outdated items, and refresh them
            foreach (var id in CachedStats.Keys.Where(id => DateTime.Now.Subtract(CachedStats[id].LastUpdated).Days >= 1))
            {
                await LoadStatsFromWeb(id);
            }
        }
    }

    /// <summary>
    /// Holds statistics result for an account
    /// </summary>
    public sealed class UserGameStat
    {
        public Dictionary<string, string> Vars = new();
        /// <summary>
        /// Statistic name:value pairs
        /// </summary>
        public Dictionary<string, string> Collected = new();
        /// <summary>
        /// Keys on this list should NOT be displayed under accounts.
        /// </summary>
        public List<string> HiddenMetrics = new();
        public DateTime LastUpdated;
    }
}
