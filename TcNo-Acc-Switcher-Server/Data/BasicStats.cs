using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using HtmlAgilityPack;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SteamKit2.GC.CSGO.Internal;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using String = System.String;

namespace TcNo_Acc_Switcher_Server.Data
{
    public sealed class BasicStats
    {
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
                    BasicStatsInit();
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

        public static Dictionary<string, Dictionary<string, string>> GetUserStatsAllGames(string platform, string accountId)
        {
            var returnDict = new Dictionary<string, Dictionary<string, string>>( );
            // Foreach available game
            foreach (var availableGame in GetAvailableGames(platform))
            {
                // That has the requested account
                if (GameStats[availableGame].CachedStats.ContainsKey(accountId))
                {
                    // Add short game identifier (for displaying) and stats to dictionary to return
                    returnDict[GameStats[availableGame].Indicator] = GameStats[availableGame].CachedStats[accountId].Collected;
                }
            }

            return returnDict;
        }
        /// <summary>
        /// List of possible games on X platform
        /// </summary>
        [JSInvokable]
        public static List<string> GetAvailableGames(string platform) => PlatformGames.ContainsKey(platform) ? PlatformGames[platform] : new List<string>();
        /// <summary>
        /// List of games from X platform, with Y accountId associated.
        /// </summary>
        [JSInvokable]
        public static List<string> GetEnabledGames(string platform, string accountId) => GetAvailableGames(platform).Where(game => GameStats[game].CachedStats.ContainsKey(accountId)).ToList();
        /// <summary>
        /// List of games from X platform, NOT with Y accountId associated.
        /// </summary>
        [JSInvokable]
        public static List<string> GetDisabledGames(string platform, string accountId) => GetAvailableGames(platform).Where(game => !GameStats[game].CachedStats.ContainsKey(accountId)).ToList();

        [JSInvokable]
        public static Dictionary<string, string> GetRequiredVars(string game) => GameStats[game].RequiredVars;

        [JSInvokable]
        public static void SetGameVars(string game, string accountId,
            Dictionary<string, string> returnDict)
        {
            GameStats[game].SetAccount(accountId, returnDict);
            GameStats[game].SaveStats();
        }

        [JSInvokable]
        public static void DisableGame(string game, string accountId)
        {
            if (!GameStats.ContainsKey(game) || !GameStats[game].CachedStats.ContainsKey(accountId)) return;

            GameStats[game].CachedStats.Remove(accountId);
            GameStats[game].SaveStats();
        }
        public static bool PlatformHasAnyGames(string platform) => PlatformGames.ContainsKey(platform) && PlatformGames[platform].Count > 0;
        public static JToken StatsDefinitions => (JObject)JData["StatsDefinitions"];
        public static JToken PlatformCompatibilities => (JObject)JData["PlatformCompatibilities"];
        public static JObject GetGame(string game) => (JObject) StatsDefinitions![game];
        public static bool GameExists(string game) => GamesDict.Contains(game);
        public static readonly string BasicStatsPath = Path.Join(Globals.AppDataFolder, "GameStats.json");

        /// <summary>
        /// Read BasicStats.json and collect game definitions, as well as platform-game relations.
        /// </summary>
        private static void BasicStatsInit()
        {
            // Check if Platforms.json exists.
            // If it doesnt: Copy it from the programs' folder to the user data folder.
            if (!File.Exists(BasicStatsPath))
            {
                // Once again verify the file exists. If it doesn't throw an error here.
                _ = GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_FailedStatsLoad"], renderTo: "toastarea");
                Globals.WriteToLog("Failed to locate GameStats.json! This will cause a lot of stats to break.");
                return;
            }

            JData = GeneralFuncs.LoadSettings(BasicStatsPath);

            // Populate the List of all Games with stats
            GamesDict = StatsDefinitions?.ToObject<Dictionary<string, object>>()?.Keys.ToList();
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
        }

        public static List<string> PlatformGamesWithStats(string platform) => PlatformGames.ContainsKey(platform) ? GetAvailableGames(platform) : new List<string>();

        /// <summary>
        /// Loads games and stats for requested platform
        /// </summary>
        /// <returns>If successfully loaded platform</returns>
        public static bool SetCurrentPlatform(string platform)
        {
            if (!PlatformGames.ContainsKey(platform)) return false;

            GameStats = new Dictionary<string, GameStat>();
            // TODO: Verify this works as intended when more games are added.
            foreach (var game in GetAvailableGames(platform))
            {
                var gs = new GameStat();
                gs.SetGameStat(game);
                GameStats.Add(game, gs);
            }

            return true;
        }

        /// <summary>
        /// Returns a string with all the statistics available for the specified account
        /// </summary>
        public static string GetGameStatsString(string accountId)
        {
            var outputString = "";
            // Foreach game in platform
            foreach (var gs in GameStats)
            {
                // Check to see if it contains the requested accountID
                var cachedStats = gs.Value.CachedStats;
                if (!cachedStats.ContainsKey(accountId)) continue;

                outputString += $"{gs.Key}: {Environment.NewLine}";

                // Add each stat from account to the string, starting with the game name.
                var collectedStats = cachedStats["accountId"].Collected;
                foreach (var stat in collectedStats)
                {
                    outputString += $"{stat.Key}: {stat.Value}{Environment.NewLine}";
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
        #region Properties

        private DateTime _lastLoadingNotification = DateTime.MinValue;

        public bool IsInit;
        public string Game;
        public string Indicator = "";
        public string Url = "";
        public string Cookies = "";
        /// <summary>
        ///Dictionary of Variable:Values for each account, for use in statistics web url. Such as Username="Kevin".
        /// </summary>
        public Dictionary<string, string> RequiredVars = new();
        /// <summary>
        /// Dictionary of GameName:CollectInstructions for collecting stats from web and info from user
        /// </summary>
        public Dictionary<string, CollectInstruction> ToCollect = new();

        private readonly string _cacheFileFolder = Path.Join(Globals.UserDataFolder, "StatsCache");
        private string CacheFilePath => Path.Join(_cacheFileFolder, $"{Game}.json");
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

            // Optional
            public string SelectAttribute { get; set; } // If Select = "attribute", set the attribute to get here.
            public string NoDisplayIf { get; set; } = ""; // The DisplayAs text will not display if equal to the default value of this.
        }

        public void SetGameStat(string game)
        {
            if (!BasicStats.GameExists(game)) return;

            var jGame = BasicStats.GetGame(game);

            Game = game;
            Indicator = (string)jGame["Indicator"];
            Url = (string)jGame["Url"];
            Cookies = (string)jGame["RequestCookies"];

            RequiredVars = jGame["Vars"]?.ToObject<Dictionary<string, string>>();
            //foreach (var (k, v) in jGame["Vars"]?.ToObject<Dictionary<string, string>>()!)
            //{
            //    Vars.Add(k, v);
            //}

            ToCollect = jGame["Collect"]?.ToObject<Dictionary<string, CollectInstruction>>();

            //foreach (var (k, v) in jGame["Collect"]?.ToObject<Dictionary<string, CollectInstruction>>()!)
            //{
            //    Collect.Add(k, v);
            //}

            LoadStats();

            IsInit = true;
        }

        /// <summary>
        /// Set up new accounts
        /// </summary>
        /// <returns>True if added, False if exists</returns>
        public bool SetAccount(string accountId, Dictionary<string, string> vars)
        {
            if (BasicStats.GameStats[Game].CachedStats.ContainsKey(accountId))
                return false;

            BasicStats.GameStats[Game].CachedStats[accountId] = new UserGameStat() { Vars = vars };

            LoadStatsFromWeb(accountId);
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
        public bool LoadStatsFromWeb(string accountId)
        {
            // Check if init, and list of accounts contains Id
            if (!IsInit || !CachedStats.ContainsKey(accountId)) return false;
            var userStat = CachedStats[accountId];

            // Check if account has required variables set.
            if (userStat.Vars.Count == 0) return false;

            // Notify user than an account is being loaded - >5 seconds apart.
            if (DateTime.Now.Subtract(_lastLoadingNotification).Seconds >= 5)
            {
                _ = GeneralInvocableFuncs.ShowToast("info", Lang.Instance["Toast_LoadingStats"], renderTo: "toastarea");
                _lastLoadingNotification = DateTime.Now;
            }

            // Read doc from web
            HtmlDocument doc = new();
            if (!Globals.GetWebHtmlDocument(ref doc, UrlSubbed(userStat), out var responseText, Cookies))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_GameStatsLoadFail", new { Game }], renderTo: "toastarea");
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
                    "attribute" => xPathResponse.Attributes["src"]?.Value ?? "",
                    _ => text
                };

                // Don't display if equal to default
                if (text == ci.NoDisplayIf) continue;

                // Else, format as requested
                text = ci.DisplayAs.Replace("%x%", text);
                userStat.Collected.Add(itemName, text);
            }

            userStat.LastUpdated = DateTime.Now;

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
        public void LoadStats()
        {
            _ = Directory.CreateDirectory(_cacheFileFolder);

            if (!File.Exists(CacheFilePath)) return;
            var jCachedStats = GeneralFuncs.LoadSettings(CacheFilePath);
            CachedStats = jCachedStats.ToObject<SortedDictionary<string, UserGameStat>>() ?? new SortedDictionary<string, UserGameStat>();

            // Check for outdated items, and refresh them
            foreach (var id in CachedStats.Keys.Where(id => DateTime.Now.Subtract(CachedStats[id].LastUpdated).Days >= 1))
            {
                LoadStatsFromWeb(id);
            }
        }
    }

    /// <summary>
    /// Holds statistics result for an account
    /// </summary>
    public sealed class UserGameStat
    {
        public Dictionary<string, string> Vars = new();
        public Dictionary<string, string> Collected = new();
        public DateTime LastUpdated;
    }
}
