using System;
using System.Collections.Generic;
using System.IO;
using System.Net;
using System.Net.Http;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppStats
    {
        // --------------------
        // GOALS:
        // --------------------
        // To collect statistics about the program.
        // A unique ID should be generated for each person. On "clear" a new ID generated.
        // ID should be used in place of IP in error reports, where available, for anonymous correlation.
        // These are to remain as anonymous as possible. I don't need nor want to collect personal info. Just info to improve the app.
        // Stats are submitted on a background thread once a day on app launch.
        // Stats are saved on program (server) close. I'm not too sure about on crash.

        // --------------------
        // STATS TO COLLECT:
        // --------------------
        // USER:
        // - Country (Based on IP, collected on submission).
        // - Number of launches
        // - Overall number of crash reports submitted
        // - First launch datetime (since stats enabled)

        // PAGE STATS:
        // - Time spent on specific pages
        // - Number of visits to each page
        // (Both of the above used for finding out which platforms are most used, etc)

        // SWITCHER (Per platform):
        // - Number of accounts
        // - Switches
        // - Unique days platform switcher used (For switches/day stats, for each platform)
        // - First and Last active days

        private static AppStats _instance = new();

        private static readonly object LockObj = new();

        public static AppStats Instance
        {
            get
            {
                lock (LockObj)
                {
                    // Load settings if have changed, or not set
                    if (_instance is { _currentlyModifying: true }) return _instance;
                    if (_instance != new AppStats() && Globals.GetFileMd5(SettingsFile) == _instance._lastHash) return _instance;

                    _instance = new AppStats { _currentlyModifying = true };

                    if (File.Exists(SettingsFile))
                    {
                        _instance = JsonConvert.DeserializeObject<AppStats>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
                        if (_instance == null)
                        {
                            _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedLoadStats"]);
                            if (File.Exists(SettingsFile))
                                Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
                            _instance = new AppStats { _currentlyModifying = true };
                        }
                        _instance._lastHash = Globals.GetFileMd5(SettingsFile);
                    }
                    else
                    {
                        SaveSettings();
                    }

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

        private static readonly Lang Lang = Lang.Instance;
        private string _lastHash = "";
        private bool _currentlyModifying;
        public static readonly string SettingsFile = "Statistics.json";

        public static void SaveSettings()
        {
            GenerateTotals();
            GeneralFuncs.SaveSettings(SettingsFile, Instance);
        }

        [JSInvokable]
        public static string GetStatsString()
        {
            if (!AppSettings.StatsEnabled) return Lang["Stats_Disabled"];
            GenerateTotals();
            SaveSettings();
            var returnString = $@"<h3>TcNo Account Switcher Stats - {DateTime.Now:yyyy-MM-dd HH:mm:ss}</h3><br>" +
                (AppSettings.StatsShare ? Lang["Stats_ShareEnabled", new { WebsiteLink = "OpenLinkInBrowser('https://tcno.co/Projects/AccSwitcher/stats/')" }] : Lang["Stats_ShareDisabled", new { WebsiteLink = "https://tcno.co/Projects/AccSwitcher/stats/" }]) +
                @$"<pre><br>
<b>{Lang["Stats_OperatingSystem"]}</b>{OperatingSystem}
<b>{Lang["Stats_FirstLaunch"]}</b>{FirstLaunch.ToString("yyyy-MM-dd HH:mm:ss")}
<b>{Lang["Stats_LaunchCount"]}</b>{LaunchCount}
<b>{Lang["Stats_CrashCount"]}</b>{CrashCount}
<b>{Lang["Stats_MostUsedPlatform"]}</b>{MostUsedPlatform}
<b>{Lang["Stats_TotalTime"]}</b>{TimeSpan.FromSeconds(PageStats["_Total"].TotalTime).ToString("hh\\:mm\\:ss")}
<b>{Lang["Stats_TotalSwitches"]}</b>{SwitcherStats["_Total"].Switches}
<b>{Lang["Stats_TotalGamesLaunched"]}</b>{SwitcherStats["_Total"].GamesLaunched}
<b>{Lang["Stats_UniqueDaysLong"]}</b>{SwitcherStats["_Total"].UniqueDays}
<br></pre>";

            return $"{returnString}<h4>{Lang["Stats_InDepth"]}</h4><pre><br>{GetInDepthStatsString()}</pre>";
        }

        /// <summary>
        /// This compliments GetStatsString(), but gives an in-depth breakdown per platform, etc.
        /// </summary>
        private static string GetInDepthStatsString()
        {
            //<b>{Lang[""]}</b>{}<br>
            if (!AppSettings.StatsEnabled) return "";
            var returnString = $@"<b>Uuid</b>: {Uuid}<br>
<b>{Lang["Stats_LastUpload"]}</b>{LastUpload:yyyy-MM-dd HH:mm:ss}<br>";

            if (SwitcherStats.Count > 1)
                returnString += $"<b>{Lang["Stats_SwitcherStats"]}</b><br>";
            foreach (var switcherStat in SwitcherStats)
            {
                if (switcherStat.Key == "_Total") continue;
                returnString += $@"- <b>{switcherStat.Key}: </b>
   - <b>{Lang["Stats_Accounts"]}</b>{switcherStat.Value.Accounts}
   - <b>{Lang["Stats_Switches"]}</b>{switcherStat.Value.Switches}
   - <b>{Lang["Stats_FirstActive"]}</b>{switcherStat.Value.FirstActive.ToString("yyyy-MM-dd HH:mm:ss")}
   - <b>{Lang["Stats_LastActive"]}</b>{switcherStat.Value.LastActive.ToString("yyyy-MM-dd HH:mm:ss")}
   - <b>{Lang["Stats_UniqueDays"]}</b>{switcherStat.Value.UniqueDays}
   - <b>{Lang["Stats_GameShortcuts"]}</b>{switcherStat.Value.GameShortcuts}
   - <b>{Lang["Stats_GameShortcutsHotbar"]}</b>{switcherStat.Value.GameShortcutsHotbar}
   - <b>{Lang["Stats_GamesLaunched"]}</b>{switcherStat.Value.GamesLaunched}<br>";
            }

            if (PageStats.Count > 1)
                returnString += $"<b>{Lang["Stats_PageStats"]}</b><br>";
            foreach (var pageStat in PageStats)
            {
                if (pageStat.Key == "_Total") continue;
                returnString += $@"- <b>{pageStat.Key}: </b>{Lang["Stats_VisitsTotalTime", new
                {
                    visits = pageStat.Value.Visits,
                    hours = TimeSpan.FromSeconds(pageStat.Value.TotalTime).ToString("hh\\:mm\\:ss")
                }]}<br>";

            }

            return returnString;
        }


        [JSInvokable]
        public static void ClearStats()
        {
            Instance = _instance = new AppStats { _currentlyModifying = true };
            SaveSettings();
        }

        public static void UploadStats()
        {
            try
            {
                // Upload stats file if enabled.
                if (!AppSettings.OfflineMode || !AppSettings.StatsEnabled || !AppSettings.StatsShare) return;
                // If not a new day
                if (LastUpload.Date == DateTime.Now.Date) return;

                // Save data in temp file.
                var tempFile = Path.GetTempFileName();
                var statsJson = JsonConvert.SerializeObject(JObject.FromObject(Instance), Formatting.None);
                File.WriteAllText(tempFile, statsJson);

                // Upload using HTTPClient
                var httpClient = new HttpClient();
                httpClient.DefaultRequestHeaders.Add("User-Agent", "TcNo Account Switcher");
                var response = httpClient.PostAsync("https://tcno.co/Projects/AccSwitcher/api/stats/",
                    new FormUrlEncodedContent(new Dictionary<string, string>
                    {
                        ["uuid"] = Uuid,
                        ["statsData"] = statsJson
                    })).Result;

                if (response.StatusCode != HttpStatusCode.OK)
                    Globals.WriteToLog("Failed to upload stats file. Status code: " + response.StatusCode);

                LastUpload = DateTime.Now;
                SaveSettings();
            } catch (Exception e)
            {
                // Ignore any errors here.
                Globals.WriteToLog(@"Could not reach https://tcno.co/ to upload statistics.", e);
            }
        }

        #region System stats
        [JsonProperty("LastUpload", Order = 1)] private DateTime _lastUpload = DateTime.MinValue;
        public static DateTime LastUpload { get => Instance._lastUpload; set => Instance._lastUpload = value; }

        [JsonProperty("OperatingSystem", Order = 2)] private string _operatingSystem;
        private static string OperatingSystem => Instance._operatingSystem ?? (Instance._operatingSystem = Globals.GetOsString());

        #endregion


        #region User stats

        [JsonProperty("Uuid", Order = 0)] private string _uuid = Guid.NewGuid().ToString();
        public static string Uuid { get => Instance._uuid; set => Instance._uuid = value; }

        [JsonProperty("LaunchCount", Order = 2)] private int _launchCount;
        public static int LaunchCount { get => Instance._launchCount; set => Instance._launchCount = value; }


        [JsonProperty("CrashCount", Order = 3)] private int _crashCount;
        public static int CrashCount { get => Instance._crashCount; set => Instance._crashCount = value; }

        [JsonProperty("FirstLaunch", Order = 4)] private DateTime _firstLaunch = DateTime.Now;
        public static DateTime FirstLaunch { get => Instance._firstLaunch; set => Instance._firstLaunch = value; }

        [JsonProperty("MostUsedPlatform", Order = 5)] private string _mostUsed = "";
        public static string MostUsedPlatform { get => Instance._mostUsed; set => Instance._mostUsed = value; }

        #endregion


        #region Page stats

        // EXAMPLE:
        // "Steam": {TotalTime: 3600, TotalVisits: 10}
        // "SteamSettings": {TotalTime: 300, TotalVisits: 6}
        [JsonProperty("PageStats", Order = 6)] private Dictionary<string, PageStat> _pageStats = new() { { "_Total", new PageStat() } };
        public static Dictionary<string, PageStat> PageStats { get => Instance._pageStats; set => Instance._pageStats = value; }

        // Total time is incremented on navigating to a new page.
        // -> Check last page, and compare times. Then add seconds.
        // This won't save, and will be lost on app restart.
        [JsonIgnore] private string _lastActivePage = "";
        public static string LastActivePage { get => Instance._lastActivePage; set => Instance._lastActivePage = value; }
        [JsonIgnore] private DateTime _lastActivePageTime = DateTime.Now;
        public static DateTime LastActivePageTime { get => Instance._lastActivePageTime; set => Instance._lastActivePageTime = value; }

        public static void NewNavigation(string newPage)
        {
            if (!AppSettings.StatsEnabled) return;

            // First page loaded, so just save current page and time.
            if (LastActivePage == "")
            {
                LastActivePage = newPage;
                LastActivePageTime = DateTime.Now;
            }

            // Else, compare and add
            if (!PageStats.ContainsKey(LastActivePage)) PageStats.Add(LastActivePage, new PageStat());
            PageStats[LastActivePage].TotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
            LastActivePage = newPage;
            LastActivePageTime = DateTime.Now;

            // Also, add to the visit count.
            if (!PageStats.ContainsKey(newPage)) PageStats.Add(newPage, new PageStat());
            PageStats[newPage].Visits++;
        }

        #endregion


        #region Switcher stats

        // EXAMPLE:
        // "Steam": {Accounts: 0, Switches: 0, Days: 0, LastActive: 2022-05-01}
        [JsonProperty("SwitcherStats", Order = 6)] private Dictionary<string, SwitcherStat> _switcherStats = new() { { "_Total", new SwitcherStat() } };
        public static Dictionary<string, SwitcherStat> SwitcherStats { get => Instance._switcherStats; set => Instance._switcherStats = value; }

        private static void AddPlatformIfNotExist(string platform)
        {
            if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
        }

        private static void IncrementSwitcherLastActive(string platform)
        {
            if (!AppSettings.StatsEnabled) return;
            // Increment unique days if day is not the same (Compares year, month, day - As we're not looking for 24 hours)
            if (SwitcherStats[platform].LastActive.Date == DateTime.Now.Date) return;
            SwitcherStats[platform].UniqueDays += 1;
            SwitcherStats[platform].LastActive = DateTime.Now;
        }

        public static void IncrementSwitches(string platform)
        {
            if (!AppSettings.StatsEnabled) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].Switches++;

            IncrementSwitcherLastActive(platform);
            AppData.RefreshDiscordPresenceAsync();
        }

        public static void SetAccountCount(string platform, int count)
        {
            if (!AppSettings.StatsEnabled) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].Accounts = count;
        }

        public static void IncrementGameLaunches(string platform)
        {
            if (!AppSettings.StatsEnabled) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].GamesLaunched++;

            IncrementSwitcherLastActive(platform);
        }

        public static void SetGameShortcutCount(string platform, Dictionary<int, string> shortcuts)
        {
            if (!AppSettings.StatsEnabled) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].GameShortcuts = shortcuts.Count;

            // Hotbar shortcuts:
            var tHShortcuts = 0;
            foreach (var (i, _) in shortcuts)
            {
                if (i < 0) tHShortcuts++;
            }
            SwitcherStats[platform].GameShortcutsHotbar = tHShortcuts;
        }

        public static void GenerateTotals()
        {
            // Account stat totals
            var totalAccounts = 0;
            var totalSwitches = 0;
            var totalGameShortcuts = 0;
            var totalGamesLaunched = 0;
            var mostUsedPlatform = "";
            var mostUsedCount = 0;
            foreach (var (k, v) in SwitcherStats)
            {
                if (k == "_Total") continue;
                totalAccounts += v.Accounts;
                totalSwitches += v.Switches;
                totalGameShortcuts += v.GameShortcuts;
                totalGamesLaunched += v.GamesLaunched;

                if (v.Switches <= mostUsedCount) continue;
                mostUsedCount = v.Switches;
                mostUsedPlatform = k;
            }

            var totStat = new SwitcherStat {
                Accounts = totalAccounts,
                Switches = totalSwitches,
                GameShortcuts = totalGameShortcuts,
                GamesLaunched = totalGamesLaunched
            };
            SwitcherStats["_Total"] = totStat;

            // Program totals
            var totalTime = 0;
            var totalVisits = 0;
            foreach (var (k, v) in PageStats)
            {
                if (k == "_Total") continue;
                totalTime += v.TotalTime;
                totalVisits += v.Visits;
            }

            var totPageStat = new PageStat { TotalTime = totalTime, Visits = totalVisits };
            PageStats["_Total"] = totPageStat;
            MostUsedPlatform = mostUsedPlatform;
        }

        #endregion

    }

    public class PageStat
    {
        public PageStat()
        {
            TotalTime = 0;
            Visits = 0;
        }
        public int TotalTime { get; set; }
        public int Visits { get; set; }
    }

    public class SwitcherStat
    {
        public SwitcherStat()
        {
            Accounts = 0;
            Switches = 0;
            UniqueDays = 1; // First day is init day
            GameShortcuts = 0;
            GameShortcutsHotbar = 0;
            GamesLaunched = 0;
            FirstActive = DateTime.Now;
            LastActive = DateTime.Now;
        }
        public int Accounts { get; set; }
        public int Switches { get; set; }
        public int UniqueDays { get; set; }
        public int GameShortcuts { get; set; }
        public int GameShortcutsHotbar { get; set; }
        public int GamesLaunched { get; set; }
        public DateTime FirstActive { get; set; }
        public DateTime LastActive { get; set; }
    }
}
