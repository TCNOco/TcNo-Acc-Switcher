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
using System.Net;
using System.Net.Http;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.Classes.Stats;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State
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
    public class Statistics : IStatistics
    {
        [Inject] private IWindowSettings WindowSettings { get; set; }
        [Inject] private IAppState AppState { get; set; }

        #region System stats
        public DateTime LastUpload { get; set; } = DateTime.MinValue;
        public string OperatingSystem { get; } = Globals.GetOsString();
        #endregion

        #region User stats
        public string Uuid { get; set; } = Guid.NewGuid().ToString();
        public int LaunchCount { get; set; }
        public int CrashCount { get; set; }
        public DateTime FirstLaunch { get; set; } = DateTime.Now;
        public string MostUsedPlatform { get; set; } = "";
        #endregion

        #region Page stats
        // EXAMPLE:
        // "Steam": {TotalTime: 3600, TotalVisits: 10}
        // "SteamSettings": {TotalTime: 300, TotalVisits: 6}
        public Dictionary<string, PageStat> PageStats { get; set; } = new() { { "_Total", new PageStat() } };

        // Total time is incremented on navigating to a new page.
        // -> Check last page, and compare times. Then add seconds.
        // This won't save, and will be lost on app restart.
        [JsonIgnore] public string LastActivePage { get; set; } = "";
        [JsonIgnore] public DateTime LastActivePageTime { get; set; } = DateTime.Now;

        public void NewNavigation(string newPage)
        {
            if (!WindowSettings.CollectStats) return;

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
        public Dictionary<string, SwitcherStat> SwitcherStats { get; set; } = new() { { "_Total", new SwitcherStat() } };

        private void AddPlatformIfNotExist(string platform)
        {
            if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
        }

        private void IncrementSwitcherLastActive(string platform)
        {
            if (!WindowSettings.CollectStats) return;
            // Increment unique days if day is not the same (Compares year, month, day - As we're not looking for 24 hours)
            if (SwitcherStats[platform].LastActive.Date == DateTime.Now.Date) return;
            SwitcherStats[platform].UniqueDays += 1;
            SwitcherStats[platform].LastActive = DateTime.Now;
        }

        public void IncrementSwitches(string platform)
        {
            if (!WindowSettings.CollectStats) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].Switches++;

            IncrementSwitcherLastActive(platform);
            AppState.Discord.RefreshDiscordPresenceAsync(false);
        }

        public void SetAccountCount(string platform, int count)
        {
            if (!WindowSettings.CollectStats) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].Accounts = count;
        }

        public void IncrementGameLaunches(string platform)
        {
            if (!WindowSettings.CollectStats) return;
            AddPlatformIfNotExist(platform);
            SwitcherStats[platform].GamesLaunched++;

            IncrementSwitcherLastActive(platform);
        }

        public void SetGameShortcutCount(string platform, Dictionary<int, string> shortcuts)
        {
            if (!WindowSettings.CollectStats) return;
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

            var totStat = new SwitcherStat
            {
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

        public void UploadStats()
        {
            try
            {
                // Upload stats file if enabled.
                if (!WindowSettings.CollectStats || !WindowSettings.ShareAnonymousStats) return;

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
                Save();
            }
            catch (Exception e)
            {
                // Ignore any errors here.
                Globals.WriteToLog(@"Could not reach https://tcno.co/ to upload statistics.", e);
            }
        }

        private const string Filename = "Statistics.json";

        public Statistics()
        {
            Globals.LoadSettings(Filename, this, false);
        }

        public void Save()
        {
            GenerateTotals();
            Globals.SaveJsonFile(Filename, this, false);
        }

        public void ClearStats()
        {
            var type = GetType();
            var properties = type.GetProperties();
            foreach (var t in properties)
                t.SetValue(this, null);
            Save();
        }
    }
}
