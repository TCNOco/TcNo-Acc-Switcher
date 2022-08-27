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
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Net.Http.Json;
using System.Text.Json.Nodes;
using System.Text;
using System.Text.RegularExpressions;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.Classes.GameStats;
using TcNo_Acc_Switcher_Server.State.Classes.Stats;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;
using static SteamKit2.GC.Dota.Internal.CMsgDetailedGameStats;

namespace TcNo_Acc_Switcher_Server.State;
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
    private readonly IWindowSettings _windowSettings;

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
    // DEPRECATED
    // TODO: This is here just for now, to translate into the new stats system. A few months from now this will be removed entirely.

    // EXAMPLE:
    // "Steam": {TotalTime: 3600, TotalVisits: 10}
    // "SteamSettings": {TotalTime: 300, TotalVisits: 6}
    [JsonProperty("PageStats")] public Dictionary<string, PageStat> OldPageStats { get; set; } = null; // TODO: When reading, set new values, and delete self.

    public OtherPageStats OtherPageStats { get; set; } = new();

    // Total time is incremented on navigating to a new page.
    // -> Check last page, and compare times. Then add seconds.
    // This won't save, and will be lost on app restart.
    [JsonIgnore] public DateTime LastActivePageTime { get; set; } = DateTime.Now;

    /// <summary>
    /// After navigation, call this. Increments the last page visits, as well as the time spent on the last page
    /// </summary>
    public void NewNavigation(string lastPage, string lastPlatform)
    {
        if (!_windowSettings.CollectStats) return;
        lastPage = lastPage.Split("?")[0]; // Remove any arguments

        // First page loaded, so just save current page and time.
        if (lastPage != "")
        {
            if (lastPage == "/")
            {
                // Is home/landing page
                OtherPageStats.Home.TotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
                OtherPageStats.Home.Visits++;
            }
            else if (lastPage.Contains("Platforms"))
            {
                OtherPageStats.Platforms.TotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
                OtherPageStats.Platforms.Visits++;
            }
            else if (lastPage.Contains("Settings"))
            {
                // Is settings page.
                var stripped = lastPage.Replace("/", "").Replace("Settings", "");
                if (stripped.Length == 0)
                {
                    // Is program's main settings
                    OtherPageStats.Settings.TotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
                    OtherPageStats.Settings.Visits++;
                }
                else
                {
                    // Is platform's settings
                    if (!SwitcherStats.ContainsKey(lastPlatform)) SwitcherStats.Add(lastPlatform, new SwitcherStat());
                    SwitcherStats[lastPlatform].SettingsTotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
                    SwitcherStats[lastPlatform].SettingsVisits++;
                }
            }
            else
            {
                // Is platform
                if (!SwitcherStats.ContainsKey(lastPlatform)) SwitcherStats.Add(lastPlatform, new SwitcherStat());
                SwitcherStats[lastPlatform].TotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
                SwitcherStats[lastPlatform].Visits++;
            }
        }

        // Having a save here works well enough, as this is called after the platform is initialized.
        LastActivePageTime = DateTime.Now;

        Save();
    }

    #endregion

    #region Switcher stats

    // EXAMPLE:
    // "Steam": {Accounts: 0, Switches: 0, Days: 0, LastActive: 2022-05-01}
    public Dictionary<string, SwitcherStat> SwitcherStats { get; set; } = new();

    private void AddPlatformIfNotExist(string platform)
    {
        if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
    }

    public void SetAccountCount(string platform, int count)
    {
        if (!_windowSettings.CollectStats) return;
        AddPlatformIfNotExist(platform);
        SwitcherStats[platform].Accounts = count;
    }

    public void IncrementSwitches(string platform)
    {
        if (!_windowSettings.CollectStats) return;
        AddPlatformIfNotExist(platform);
        SwitcherStats[platform].Switches++;

        IncrementSwitcherLastActive(platform);
    }

    public void IncrementGameLaunches(string platform)
    {
        if (!_windowSettings.CollectStats) return;
        AddPlatformIfNotExist(platform);
        SwitcherStats[platform].GamesLaunched++;

        IncrementSwitcherLastActive(platform);
    }

    private void IncrementSwitcherLastActive(string platform)
    {
        if (!_windowSettings.CollectStats) return;
        // Increment unique days if day is not the same (Compares year, month, day - As we're not looking for 24 hours)
        if (SwitcherStats[platform].LastActive.Date == DateTime.Now.Date) return;
        SwitcherStats[platform].UniqueDays += 1;
        SwitcherStats[platform].LastActive = DateTime.Now;

        Save();
    }

    public void SetGameShortcutCount(string platform, Dictionary<int, string> shortcuts)
    {
        if (!_windowSettings.CollectStats) return;
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

    #endregion

    #region Game Stats
    /// <summary>
    /// Collect game stats from StatsCache\ folder, and prepare to upload info about it.
    /// </summary>
    private void CollectGameStats()
    {
        var path = Path.Join(Globals.UserDataFolder, "StatsCache");
        if (!Directory.Exists(path)) return;

        var files = Directory.GetFiles(path);
        foreach (var f in files)
        {
            // Read file as JSON
            try
            {
                var gameDict = JsonConvert.DeserializeObject<Dictionary<string, BasicGameStatJs>>(File.ReadAllText(f));
                if (gameDict is null) continue;
                var metricsTotal = new ConcurrentDictionary<string, int>();

                foreach (var acc in gameDict)
                {
                    if (acc.Value.Metrics is null) continue;
                    foreach (var metricName in acc.Value.Metrics)
                    {
                        metricsTotal.AddOrUpdate(metricName, 1, (id, count) => count + 1);
                    }
                }

                var gameName = Path.GetFileName(f).Replace(".json", "");

                AllGameStats[gameName] = new BasicGameStats
                {
                    NumAccounts = gameDict.Count,
                    Metrics = metricsTotal.ToDictionary(x => x.Key, x => x.Value)
                };
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
                throw;
            }
        }
    }

    /// <summary>
    /// Used in deserializing cached stats
    /// </summary>
    private struct BasicGameStatJs
    {
        public List<string> Metrics;
    }

    public Dictionary<string, BasicGameStats> AllGameStats { get; set; } = new();
    #endregion

    public void UploadStats()
    {
        try
        {
            // Collect game stats
            CollectGameStats();

            // Upload stats file if enabled.
            if (!_windowSettings.CollectStats || !_windowSettings.ShareAnonymousStats) return;

            // If not a new day
            if (LastUpload.Date == DateTime.Now.Date) return;

            // Save data in temp file.
            var tempFile = Path.GetTempFileName();
            var statsJson = JsonConvert.SerializeObject(JObject.FromObject(this), Formatting.None);
            File.WriteAllText(tempFile, statsJson);
            Console.WriteLine(tempFile);

            // Upload using HTTPClient
            var httpClient = new HttpClient();
            httpClient.DefaultRequestHeaders.Add("User-Agent", "TcNo Account Switcher");
            var response = httpClient.PostAsync($"https://api.tcno.co/sw/stats/{Uuid}", new StringContent(statsJson, Encoding.UTF8, "application/json")).Result;

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

    public Statistics(IWindowSettings windowSettings)
    {
        _windowSettings = windowSettings;
        Globals.LoadSettings(Filename, this, false);
        // Increment launch count. This is only loaded once, so this is a good spot to put it.
        LaunchCount++;

        MoveToNewStats();

        // Todo?: Never thought about placing these in a few places, but may be useful to put elsewhere.
        AppDomain.CurrentDomain.ProcessExit += (_, _) => Save();
    }

    private void MoveToNewStats()
    {
        if (OldPageStats is null) return; // Already moved
        foreach (var (key, value) in OldPageStats)
        {
            if (key == "_Total") continue;
            if (key == "/")
            {
                OtherPageStats.Home.TotalTime = value.TotalTime;
                OtherPageStats.Home.Visits = value.Visits;
                continue;
            }

            if (key == "/Platforms/")
            {
                OtherPageStats.Platforms.TotalTime = value.TotalTime;
                OtherPageStats.Platforms.Visits = value.Visits;
                continue;
            }

            // Replace slashes and brackets.
            var newKey = Regex.Replace(key, @"/[\[\/\]]/g", "");

            if (newKey == "Settings")
            {
                // Is app settings
                OtherPageStats.Settings.TotalTime = value.TotalTime;
                OtherPageStats.Settings.Visits = value.Visits;
                continue;
            }

            var platform = newKey.Replace("Settings", "");

            if (newKey.Contains("Settings"))
            {
                // Is platform's settings
                if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
                SwitcherStats[platform].SettingsTotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
                SwitcherStats[platform].SettingsVisits++;
            }
            else
            {
                // Is platform
                if (!SwitcherStats.ContainsKey(platform)) SwitcherStats.Add(platform, new SwitcherStat());
                SwitcherStats[platform].TotalTime += (int)(DateTime.Now - LastActivePageTime).TotalSeconds;
                SwitcherStats[platform].Visits++;
            }
        }

        OldPageStats = null;
        Save();
    }

    public void Save() => Globals.SaveJsonFile(Filename, this, false);

    public void ClearStats()
    {
        var type = GetType();
        var properties = type.GetProperties();
        foreach (var t in properties)
            t.SetValue(this, null);
        Save();
    }
}