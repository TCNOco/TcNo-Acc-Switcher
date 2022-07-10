using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using HtmlAgilityPack;
using Newtonsoft.Json.Linq;
using SteamKit2.GC.CSGO.Internal;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;

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

        public static List<string> GamesDict { get => Instance._gamesDict; set => Instance._gamesDict = value; }
        public static SortedDictionary<string, List<string>> PlatformGames { get => Instance._platformGames; set => Instance._platformGames = value; }
        public static Dictionary<string, GameStat> GameStats { get => Instance._gameStats; set => Instance._gameStats = value; }

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

        public static List<string> PlatformGamesWithStats(string platform) => PlatformGames.ContainsKey(platform) ? PlatformGames[platform] : new List<string>();

        public static void SetCurrentPlatform(string platform)
        {
            GameStats = new Dictionary<string, GameStat>();
            foreach (var game in GamesDict)
            {
                var gs = new GameStat();
                gs.SetGameStat(game);
                GameStats.Add(game, gs);
            }
        }
    }

    /// <summary>
    /// A single game statistics definition, for use with others on a platform.
    /// </summary>
    public sealed class GameStat
    {
        #region Properties
        public bool IsInit;
        public bool IsLoaded;
        public string Game;
        public string Indicator = "";
        public string Url = "";
        public Dictionary<string, string> RequiredVars = new();
        public Dictionary<string, CollectInstruction> ToCollect = new();
        public Dictionary<string, string> Collected = new();

        // Set by user or loaded
        public Dictionary<string, string> Vars = new()
        {
            {"Username", "SweatyGoesPro" }
        };
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

            IsInit = true;
        }

        /// <summary>
        /// Return URL with saved vars subbed in
        /// </summary>
        private string UrlSubbed
        {
            get
            {
                var completeUrl = Url;
                foreach (var (key, value) in Vars)
                {
                    completeUrl = completeUrl.Replace($"{{{key}}}", value);
                }
                return completeUrl;
            }
        }

        public bool LoadStatsFromWeb()
        {
            if (!IsInit) return false;

            // Read doc from web
            HtmlDocument doc = new();
            if (!Globals.GetWebHtmlDocument(ref doc, UrlSubbed, out var responseText))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_GameStatsLoadFail", new { Game }], renderTo: "toastarea");
                return false;
            }

            return true;
            // Collect each requested element
            foreach (var (itemName, ci) in ToCollect)
            {
                var text = "";
                var xPathResponse = doc.DocumentNode.SelectSingleNode(ci.XPath);
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
                Collected.Add(itemName, text);
            }

            return true;
        }
    }
}
