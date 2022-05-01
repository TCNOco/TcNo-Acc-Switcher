using System;
using System.IO;
using System.Collections.Generic;
using Newtonsoft.Json;
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

        // DESIGN:
        // Array [] = [
        //      "Steam": {},
        // ]

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
                        _instance = JsonConvert.DeserializeObject<AppStats>(File.ReadAllText(SettingsFile), new JsonSerializerSettings() { });
                        if (_instance == null)
                        {
                            _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedLoadStats"]);
                            if (File.Exists(SettingsFile))
                                Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"), true);
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
            set => _instance = value;
        }

        private static readonly Lang Lang = Lang.Instance;
        private string _lastHash = "";
        private bool _currentlyModifying = false;
        public static readonly string SettingsFile = "Statistics.json";
        public static void SaveSettings() => GeneralFuncs.SaveSettings(SettingsFile, Instance);

        #region User stats

        [JsonProperty("LaunchCount")] private int _launchCount = 0;
        public static int LaunchCount { get => Instance._launchCount; set => Instance._launchCount = value; }


        [JsonProperty("CrashCount")] private int _crashCount = 0;
        public static int CrashCount { get => Instance._crashCount; set => Instance._crashCount = value; }

        [JsonProperty("FirstLaunch")] private DateTime _firstLaunch = DateTime.Now;
        public static DateTime FirstLaunch { get => Instance._firstLaunch; set => Instance._firstLaunch = value; }

        #endregion

        #region Page stats
        // EXAMPLE:
        // "Steam": {TotalTime: 3600, TotalVisits: 10}
        // "SteamSettings": {TotalTime: 300, TotalVisits: 6}
        [JsonProperty("PageStats")] private Dictionary<string, PageStat> _pageStats = new();
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

            SaveSettings();
        }
        #endregion

        #region Switcher stats
        // EXAMPLE:
        // "Steam": {Accounts: 0, Switches: 0, Days: 0, LastActive: 2022-05-01}
        [JsonProperty("SwitcherStats")] private Dictionary<string, SwitcherStat> _switcherStats = new();
        public static Dictionary<string, SwitcherStat> SwitcherStats { get => Instance._switcherStats; set => Instance._switcherStats = value; }

        public static void IncrementSwitches(string platform)
        {
            if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
            SwitcherStats[platform].Switches++;

            // Increment unique days if day is not the same (Compares year, month, day - As we're not looking for 24 hours)
            if (SwitcherStats[platform].LastActive.ToShortDateString() != DateTime.Now.ToShortDateString())
            {
                SwitcherStats[platform].UniqueDays += 1;
                SwitcherStats[platform].LastActive = DateTime.Now;
            }

            SaveSettings();
        }

        public static void SetAccountCount(string platform, int count)
        {
            if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
            SwitcherStats[platform].Accounts = count;

            SaveSettings();
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
            UniqueDays = 0;
            FirstActive = DateTime.Now;
            LastActive = DateTime.Now;
        }
        public int Accounts { get; set; }
        public int Switches { get; set; }
        public int UniqueDays { get; set; }
        public DateTime FirstActive { get; set; }
        public DateTime LastActive { get; set; }
    }
}
