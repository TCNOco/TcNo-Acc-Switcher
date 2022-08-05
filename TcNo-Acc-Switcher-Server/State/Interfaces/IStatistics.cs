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
using TcNo_Acc_Switcher_Server.State.Classes.GameStats;
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
    void SetAccountCount(string platform, int count);
    void IncrementSwitches(string platform);
    void IncrementGameLaunches(string platform);
    void UpdateGameStats(string platform, Dictionary<string, GameStatSaved> savedStats);
    void SetGameShortcutCount(string platform, Dictionary<int, string> shortcuts);
    void GenerateTotals();
    void UploadStats();
    void Save();
    void ClearStats();
}