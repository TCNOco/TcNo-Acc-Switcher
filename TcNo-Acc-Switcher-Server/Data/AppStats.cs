using System;
using System.Collections.Generic;
using System.IO;
using System.Net;
using System.Net.Http;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Data
{
    public interface IAppStats
    {
        void SaveSettings();
        void ClearStats();
        void UploadStats();
        DateTime LastUpload { get; set; }
        string OperatingSystem { get; }
        string Uuid { get; set; }
        int LaunchCount { get; set; }
        int CrashCount { get; set; }
        DateTime FirstLaunch { get; set; }
        string MostUsedPlatform { get; set; }
        Dictionary<string, PageStat> PageStats { get; set; }
        string LastActivePage { get; set; }
        DateTime LastActivePageTime { get; set; }
        Dictionary<string, SwitcherStat> SwitcherStats { get; set; }
        void NewNavigation(string newPage);
        void IncrementSwitches(string platform);
        void SetAccountCount(string platform, int count);
        void IncrementGameLaunches(string platform);
        void SetGameShortcutCount(string platform, Dictionary<int, string> shortcuts);
        void GenerateTotals();
    }

    public class AppStats : IAppStats
    {
        private readonly IAppData _appData;
        private readonly IAppSettings _appSettings;

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

        public AppStats(IAppData appData, IAppSettings appSettings, ILang lang, IGeneralFuncs generalFuncs, bool fresh = false)
        {
            _appData = appData;
            _appSettings = appSettings;
            try
            {
                if (!fresh) JsonConvert.PopulateObject(File.ReadAllText(SettingsFile), this);
                //_instance = JsonConvert.DeserializeObject<AppSettings>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to load AppStats", e);
                _ = generalFuncs.ShowToast("error", lang["Toast_FailedLoadStats"]);
                if (File.Exists(SettingsFile))
                    Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
            }

            // Increment launch count.
            LaunchCount++;
        }

        public readonly string SettingsFile = "Statistics.json";

        public void SaveSettings()
        {
            GenerateTotals();
            Data.GeneralFuncs.SaveSettings(SettingsFile, this);
        }


        public void ClearStats()
        {
            var type = GetType();
            var properties = type.GetProperties();
            foreach (var t in properties)
                t.SetValue(this, null);

            SaveSettings();
        }

        public void UploadStats()
        {
            try
            {
                // Upload stats file if enabled.
                if (!_appSettings.StatsEnabled || !_appSettings.StatsShare) return;
                // If not a new day
                if (LastUpload.Date == DateTime.Now.Date) return;

                // Save data in temp file.
                var tempFile = Path.GetTempFileName();
                var statsJson = JsonConvert.SerializeObject(JObject.FromObject(this), Formatting.None);
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
        [JsonProperty("LastUpload", Order = 1)] public DateTime LastUpload { get; set; } = DateTime.MinValue;
        [JsonProperty("OperatingSystem", Order = 2)] public string OperatingSystem => Globals.GetOsString();
        #endregion


        #region User stats
        [JsonProperty(Order = 0)] public string Uuid { get; set; } = "";
        [JsonProperty(Order = 2)] public int LaunchCount { get; set; }
        [JsonProperty(Order = 3)] public int CrashCount { get; set; }
        [JsonProperty(Order = 4)] public DateTime FirstLaunch { get; set; } = DateTime.Now;
        [JsonProperty(Order = 5)] public string MostUsedPlatform { get; set; } = "";

        #endregion


        #region Page stats

        // EXAMPLE:
        // "Steam": {TotalTime: 3600, TotalVisits: 10}
        // "SteamSettings": {TotalTime: 300, TotalVisits: 6}
        [JsonProperty(Order = 6)] public Dictionary<string, PageStat> PageStats { get; set; } = new() { { "_Total", new PageStat() } };

        // Total time is incremented on navigating to a new page.
        // -> Check last page, and compare times. Then add seconds.
        // This won't save, and will be lost on app restart.
        [JsonIgnore] public string LastActivePage { get; set; } = "";
        [JsonIgnore] public DateTime LastActivePageTime { get; set; } = DateTime.Now;

        public void NewNavigation(string newPage)
        {
            if (!_appSettings.StatsEnabled) return;

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
         [JsonProperty(Order = 6)] public Dictionary<string, SwitcherStat> SwitcherStats { get; set; } = new() { { "_Total", new SwitcherStat() } };

        private void AddPlatformIfNotExist(string platform)
        {
            if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
        }

        private void IncrementSwitcherLastActive(string platform)
        {
            if (!_appSettings.StatsEnabled) return;
            // Increment unique days if day is not the same (Compares year, month, day - As we're not looking for 24 hours)
            if (SwitcherStats[platform].LastActive.Date == DateTime.Now.Date) return;
            SwitcherStats[platform].UniqueDays += 1;
            SwitcherStats[platform].LastActive = DateTime.Now;
        }

        public void IncrementSwitches(string platform)
        {
            if (!_appSettings.StatsEnabled) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].Switches++;

            IncrementSwitcherLastActive(platform);
            _appData.RefreshDiscordPresenceAsync(false);
        }

        public void SetAccountCount(string platform, int count)
        {
            if (!_appSettings.StatsEnabled) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].Accounts = count;
        }

        public void IncrementGameLaunches(string platform)
        {
            if (!_appSettings.StatsEnabled) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].GamesLaunched++;

            IncrementSwitcherLastActive(platform);
        }

        public void SetGameShortcutCount(string platform, Dictionary<int, string> shortcuts)
        {
            if (!_appSettings.StatsEnabled) return;
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

        public void GenerateTotals()
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
