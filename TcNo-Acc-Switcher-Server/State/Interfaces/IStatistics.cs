using System;
using System.Collections.Generic;
using TcNo_Acc_Switcher_Server.State.Classes.Stats;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface IStatistics
{
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
    void UploadStats();
    void Save();
    void ClearStats();
}