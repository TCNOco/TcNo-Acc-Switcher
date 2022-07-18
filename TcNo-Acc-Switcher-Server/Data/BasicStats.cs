using System;
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

namespace TcNo_Acc_Switcher_Server.Data
{
    public interface IBasicStats
    {
        /// <summary>
        /// List of all games with stats
        /// </summary>
        List<string> GamesDict { get; set; }

        /// <summary>
        /// Dictionary of Platform:List of supported game names
        /// </summary>
        SortedDictionary<string, List<string>> PlatformGames { get; set; }

        /// <summary>
        /// Dictionary of Game Name:Available user game statistics
        /// </summary>
        Dictionary<string, GameStat> GameStats { get; set; }

        JToken StatsDefinitions { get; }
        JToken PlatformCompatibilities { get; }

        UserGameStat GetUserGameStat(string game, string accountId);

        /// <summary>
        /// Get list of games for current platform - That have at least 1 account for settings.
        /// </summary>
        List<string> GetAllCurrentlyEnabledGames();

        /// <summary>
        /// Get and return icon html markup for specific game
        /// </summary>
        string GetIcon(string game, string statName);

        /// <summary>
        /// Returns list Dictionary of Game Names:[Dictionary of statistic names and StatValueAndIcon (values and indicator text for HTML)]
        /// </summary>
        Dictionary<string, Dictionary<string, BasicStats.StatValueAndIcon>> GetUserStatsAllGamesMarkup(string platform, string accountId);

        /// <summary>
        /// List of possible games on X platform
        /// </summary>
        List<string> GetAvailableGames(string platform);

        /// <summary>
        /// List of games from X platform, with Y accountId associated.
        /// </summary>
        List<string> GetEnabledGames(string platform, string accountId);

        /// <summary>
        /// List of games from X platform, NOT with Y accountId associated.
        /// </summary>
        List<string> GetDisabledGames(string platform, string accountId);

        Dictionary<string, string> GetRequiredVars(string game);
        Dictionary<string, string> GetExistingVars(string game, string account);

        /// <summary>
        /// Gets list of all metric names to collect for the provided account, as well as whether each is hidden or not, and the text to display in the UI checkbox.
        /// </summary>
        Dictionary<string, Tuple<bool, string>> GetHiddenMetrics(string game, string account);

        /// <summary>
        /// Get list of metrics that are set to hidden from the settings menu. This overrides individual account settings.
        /// </summary>
        List<string> GetGloballyHiddenMetrics(string game);

        /// <summary>
        /// Gets list of all metric names to collect, as well as whether each is hidden or not, and the text to display in the UI checkbox.
        /// Example: "AP":"Arena Points"
        /// </summary>
        //public Dictionary<string, Tuple<bool, string>> GetAllMetrics(string game)
        Dictionary<string, string> GetAllMetrics(string game);

        Task<bool> SetGameVars(string platform, string game, string accountId, Dictionary<string, string> returnDict, List<string> hiddenMetrics);
        void DisableGame(string game, string accountId);
        Task RefreshAccount(string accountId, string game, string platform = "");
        bool PlatformHasAnyGames(string platform);
        JObject GetGame(string game);
        bool GameExists(string game);

        /// <summary>
        /// Get longer game name from it's short unique ID.
        /// </summary>
        string GetGameNameFromId(string id);

        /// <summary>
        /// Get short unique ID from game name.
        /// </summary>
        string GetGameIdFromName(string name);

        Task RefreshAllAccounts(string game, string platform = "");
        List<string> PlatformGamesWithStats(string platform);

        /// <summary>
        /// Loads games and stats for requested platform
        /// </summary>
        /// <returns>If successfully loaded platform</returns>
        Task<bool> SetCurrentPlatform(string platform);

        /// <summary>
        /// Returns a string with all the statistics available for the specified account
        /// </summary>
        string GetGameStatsString(string accountId, string sep, bool isBasic = false);
    }

    public sealed class BasicStats : IBasicStats
    {
        private readonly ILang _lang;
        private readonly IAppSettings _appSettings;
        private readonly IAppData _appData;
        private readonly IGeneralFuncs _generalFuncs;
        private readonly IBasicStats _basicStats;

        public BasicStats(ILang lang, IAppSettings appSettings, IAppData appData, IGeneralFuncs generalFuncs, IBasicStats basicStats)
        {
            _lang = lang;
            _appSettings = appSettings;
            _appData = appData;
            _generalFuncs = generalFuncs;
            _basicStats = basicStats;
            BasicStatsInit().GetAwaiter().GetResult();
        }

        private JObject JData { get; set; }

        /// <summary>
        /// List of all games with stats
        /// </summary>
        public List<string> GamesDict { get; set; } = new();
        /// <summary>
        /// Dictionary of Platform:List of supported game names
        /// </summary>
        public SortedDictionary<string, List<string>> PlatformGames { get; set; } = new();
        /// <summary>
        /// Dictionary of Game Name:Available user game statistics
        /// </summary>
        public Dictionary<string, GameStat> GameStats { get; set; } = new();

        public UserGameStat GetUserGameStat(string game, string accountId) =>
            GameStats[game].CachedStats.ContainsKey(accountId) ? GameStats[game].CachedStats[accountId] : null;

        /// <summary>
        /// Get list of games for current platform - That have at least 1 account for settings.
        /// </summary>
        public List<string> GetAllCurrentlyEnabledGames()
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
        public string GetIcon(string game, string statName) => GameStats[game].ToCollect[statName].Icon;

        public class StatValueAndIcon
        {
            public string StatValue { get; set; }
            public string IndicatorMarkup { get; set; }
        }

        /// <summary>
        /// Returns list Dictionary of Game Names:[Dictionary of statistic names and StatValueAndIcon (values and indicator text for HTML)]
        /// </summary>
        public Dictionary<string, Dictionary<string, StatValueAndIcon>> GetUserStatsAllGamesMarkup(string platform, string accountId)
        {
            var returnDict = new Dictionary<string, Dictionary<string, StatValueAndIcon>>( );
            // Foreach available game
            foreach (var availableGame in GetAvailableGames(platform))
            {
                if (!GameStats.ContainsKey(availableGame)) continue;

                // That has the requested account
                if (!GameStats[availableGame].CachedStats.ContainsKey(accountId)) continue;
                var gameIndicator = GameStats[availableGame].Indicator;
                //var gameUniqueId = GameStats[availableGame].UniqueId;

                var statValueIconDict = new Dictionary<string, StatValueAndIcon>();

                // Add icon or identifier to stat pair for displaying
                foreach (var (statName, statValue) in GameStats[availableGame].CachedStats[accountId].Collected)
                {
                    if (GameStats[availableGame].CachedStats[accountId].HiddenMetrics.Contains(statName)) continue;

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
        /// List of possible games on X platform
        /// </summary>
        [JSInvokable]
        public List<string> GetAvailableGames(string platform) => PlatformGames.ContainsKey(platform) ? PlatformGames[platform] : new List<string>();
        /// <summary>
        /// List of games from X platform, with Y accountId associated.
        /// </summary>
        [JSInvokable]
        public List<string> GetEnabledGames(string platform, string accountId) => GetAvailableGames(platform).Where(game => GameStats[game].CachedStats.ContainsKey(accountId)).ToList();
        /// <summary>
        /// List of games from X platform, NOT with Y accountId associated.
        /// </summary>
        [JSInvokable]
        public List<string> GetDisabledGames(string platform, string accountId) => GetAvailableGames(platform).Where(game => !GameStats[game].CachedStats.ContainsKey(accountId)).ToList();

        [JSInvokable]
        public Dictionary<string, string> GetRequiredVars(string game) => GameStats[game].RequiredVars;
        [JSInvokable]
        public Dictionary<string, string> GetExistingVars(string game, string account) => GameStats[game].CachedStats.ContainsKey(account) ? GameStats[game].CachedStats[account].Vars : new Dictionary<string, string>();

        /// <summary>
        /// Gets list of all metric names to collect for the provided account, as well as whether each is hidden or not, and the text to display in the UI checkbox.
        /// </summary>
        [JSInvokable]
        public Dictionary<string, Tuple<bool, string>> GetHiddenMetrics(string game, string account)
        {
            var returnDict = new Dictionary<string, Tuple<bool, string>>();
            foreach (var (key, _) in GameStats[game].ToCollect)
            {
                var hidden = GameStats[game].CachedStats.ContainsKey(account) && GameStats[game].CachedStats[account].HiddenMetrics.Contains(key);
                var text = GameStats[game].ToCollect[key].ToggleText;
                returnDict.Add(key, new Tuple<bool, string>(hidden, text));
            }

            return returnDict;
        }

        /// <summary>
        /// Get list of metrics that are set to hidden from the settings menu. This overrides individual account settings.
        /// </summary>
        [JSInvokable]
        public List<string> GetGloballyHiddenMetrics(string game) =>
            (from metric in _appSettings.GloballyHiddenMetrics[game] where metric.Value select metric.Key).ToList();

        /// <summary>
        /// Gets list of all metric names to collect, as well as whether each is hidden or not, and the text to display in the UI checkbox.
        /// Example: "AP":"Arena Points"
        /// </summary>
        //public Dictionary<string, Tuple<bool, string>> GetAllMetrics(string game)
        public Dictionary<string, string> GetAllMetrics(string game)
        {
            //// Get hidden metrics for this game
            //var hiddenMetrics = new List<string>();
            //if (AppSettings.GloballyHiddenMetrics.ContainsKey(game))
            //    hiddenMetrics = AppSettings.GloballyHiddenMetrics[game];

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

        [JSInvokable]
        public async Task<bool> SetGameVars(string platform, string game, string accountId, Dictionary<string, string> returnDict, List<string> hiddenMetrics) =>
            await GameStats[game].SetAccount(accountId, returnDict, hiddenMetrics, platform);

        [JSInvokable]
        public void DisableGame(string game, string accountId)
        {
            if (!GameStats.ContainsKey(game) || !GameStats[game].CachedStats.ContainsKey(accountId)) return;

            GameStats[game].CachedStats.Remove(accountId);
            GameStats[game].SaveStats();
        }

        [JSInvokable]
        public async Task RefreshAccount(string accountId, string game, string platform = "")
        {
            await GameStats[game].LoadStatsFromWeb(accountId, platform);
            GameStats[game].SaveStats();
        }

        public bool PlatformHasAnyGames(string platform) => platform is not null && (PlatformGames.ContainsKey(platform) && PlatformGames[platform].Count > 0);
        public JToken StatsDefinitions => (JObject)JData["StatsDefinitions"];
        public JToken PlatformCompatibilities => (JObject)JData["PlatformCompatibilities"];
        public JObject GetGame(string game) => (JObject) StatsDefinitions![game];
        public bool GameExists(string game) => GamesDict.Contains(game);
        public readonly string BasicStatsPath = Path.Join(Globals.AppDataFolder, "GameStats.json");

        /// <summary>
        /// Read BasicStats.json and collect game definitions, as well as platform-game relations.
        /// </summary>
        private async Task BasicStatsInit()
        {
            // Check if Platforms.json exists.
            // If it doesnt: Copy it from the programs' folder to the user data folder.
            if (!File.Exists(BasicStatsPath))
            {
                // Once again verify the file exists. If it doesn't throw an error here.
                await _generalFuncs.ShowToast("error", _lang["Toast_FailedStatsLoad"], renderTo: "toastarea");
                Globals.WriteToLog("Failed to locate GameStats.json! This will cause a lot of stats to break.");
                return;
            }

            JData = _generalFuncs.LoadSettings(BasicStatsPath);

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
                if (!_appSettings.GloballyHiddenMetrics.ContainsKey(game))
                    _appSettings.GloballyHiddenMetrics.Add(game, new Dictionary<string, bool>());

                var allMetrics = statsDefinitions?[game]["Collect"]?.ToObject<Dictionary<string, JObject>>();
                if (allMetrics is null) continue;
                foreach (var key in allMetrics.Keys)
                {
                    // Add to list if not there already.
                    if (!_appSettings.GloballyHiddenMetrics[game].ContainsKey(key))
                        _appSettings.GloballyHiddenMetrics[game].Add(key, false);

                }

                //// For each game's setting, make sure it exists if not already set.
                //foreach (var gameMetrics in GetAllMetrics(game))
                //{
                //    // Add to list if not there already.
                //    if (!AppSettings.GloballyHiddenMetrics[game].ContainsKey(gameMetrics.Key))
                //        AppSettings.GloballyHiddenMetrics[game].Add(gameMetrics.Key, false);
                //}

            }
        }

        /// <summary>
        /// Get longer game name from it's short unique ID.
        /// </summary>
        public string GetGameNameFromId(string id) => GameStats.FirstOrDefault(x => x.Value.UniqueId.Equals(id, StringComparison.OrdinalIgnoreCase)).Key;
        /// <summary>
        /// Get short unique ID from game name.
        /// </summary>
        public string GetGameIdFromName(string name) => GameStats.FirstOrDefault(x => x.Key.Equals(name, StringComparison.OrdinalIgnoreCase)).Value.UniqueId;

        public async Task RefreshAllAccounts(string game, string platform = "")
        {
            foreach (var id in GameStats[game].CachedStats.Keys)
            {
                await GameStats[game].LoadStatsFromWeb(id, platform);
            }

            if (game != "")
                GameStats[game].SaveStats();
        }

        public List<string> PlatformGamesWithStats(string platform) => PlatformGames.ContainsKey(platform) ? GetAvailableGames(platform) : new List<string>();

        /// <summary>
        /// Loads games and stats for requested platform
        /// </summary>
        /// <returns>If successfully loaded platform</returns>
        public async Task<bool> SetCurrentPlatform(string platform)
        {
            _appData.CurrentSwitcher = platform;
            if (!PlatformGames.ContainsKey(platform)) return false;

            GameStats = new Dictionary<string, GameStat>();
            // TODO: Verify this works as intended when more games are added.
            foreach (var game in GetAvailableGames(platform))
            {
                var gs = new GameStat(_generalFuncs, _lang, _basicStats);
                await gs.SetGameStat(game);
                GameStats.Add(game, gs);
            }

            return true;
        }

        /// <summary>
        /// Returns a string with all the statistics available for the specified account
        /// </summary>
        public string GetGameStatsString(string accountId, string sep, bool isBasic = false)
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
        private readonly IGeneralFuncs _generalFuncs;
        private readonly ILang _lang;
        private readonly IBasicStats _basicStats;

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
        public GameStat(IGeneralFuncs generalFuncs, ILang lang, IBasicStats basicStats)
        {
            _generalFuncs = generalFuncs;
            _lang = lang;
            _basicStats = basicStats;
        }

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
            if (!_basicStats.GameExists(game)) return;

            var jGame = _basicStats.GetGame(game);

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
                    collectInstruction.Value.ToggleText = _lang["ProfileImage_ToggleText"];

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
        public async Task<bool> SetAccount(string accountId, Dictionary<string, string> vars, List<string> hiddenMetrics, string platform = "")
        {
            if (CachedStats.ContainsKey(accountId))
            {
                CachedStats[accountId].Vars = vars;
                CachedStats[accountId].HiddenMetrics = hiddenMetrics;
            }
            else
                CachedStats[accountId] = new UserGameStat() { Vars = vars, HiddenMetrics = hiddenMetrics };

            if (CachedStats[accountId].Collected.Count == 0 || DateTime.Now.Subtract(CachedStats[accountId].LastUpdated).Days >= 1)
            {
                await _generalFuncs.ShowToast("info", _lang["Toast_LoadingStats"], renderTo: "toastarea");
                _lastLoadingNotification = DateTime.Now;
                return await LoadStatsFromWeb(accountId, platform);
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
                await _generalFuncs.ShowToast("info", _lang["Toast_LoadingStats"], renderTo: "toastarea");
                _lastLoadingNotification = DateTime.Now;
            }

            // Read doc from web
            HtmlDocument doc = new();
            if (!Globals.GetWebHtmlDocument(ref doc, UrlSubbed(userStat), out var responseText, Cookies))
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_GameStatsLoadFail", new { Game }], renderTo: "toastarea");
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
                        Globals.DownloadProfileImage(platform, accountId, text, _generalFuncs.WwwRoot());
                    }
                    continue;
                }

                // And special Special Types
                if (ci.SpecialType == "ImageDownload")
                {
                    var fileName = $"{Indicator}-{itemName}-{accountId}.jpg";
                    var imgPath = Path.Join(_generalFuncs.WwwRoot(), "img\\statsCache\\" + fileName);
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
                await _generalFuncs.ShowToast("error", _lang["Toast_GameStatsEmpty", new { AccoundId = Globals.GetCleanFilePath(accountId), Game = Globals.GetCleanFilePath(Game) }], renderTo: "toastarea");
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
            var jCachedStats = _generalFuncs.LoadSettings(CacheFilePath);
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
