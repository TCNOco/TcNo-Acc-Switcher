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

using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State.Classes.GameStats;
using TcNo_Acc_Switcher.State.DataTypes;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.State;

/// <summary>
/// This file stores all the instructions and information from GameStats.json
/// </summary>
public class GameStatsRoot : IGameStatsRoot
{
    public bool IsInit { get; set; }


    public Dictionary<string, GameStatDefinition> StatsDefinitions { get; set; }
    public Dictionary<string, List<string>> PlatformCompatibilities { get; set; }
    public bool GameExists(string game) => GameList.Contains(game);

    // Generated:
    public readonly string BasicStatsPath = Path.Join(Globals.AppDataFolder, "GameStats.json");
    /// <summary>
    /// List of all games with stats
    /// </summary>
    public List<string> GameList { get; set; }

    public GameStatsRoot(IToasts toasts, IWindowSettings windowSettings)
    {
        // Check if Platforms.json exists.
        // If it doesnt: Copy it from the programs' folder to the user data folder.
        if (!File.Exists(BasicStatsPath))
        {
            // Once again verify the file exists. If it doesn't throw an error here.
            toasts.ShowToastLang(ToastType.Error, "Toast_FailedStatsLoad");
            Globals.WriteToLog("Failed to locate GameStats.json! This will cause a lot of stats to break.");
            IsInit = false;
            return;
        }

        JsonConvert.PopulateObject(File.ReadAllText(BasicStatsPath), this);

        // Populate the List of all Games with stats
        GameList = StatsDefinitions.Keys.ToList();

        // Foreach game:
        foreach (var game in GameList)
        {
            // If game not on global settings list, add it to the list.
            if (!windowSettings.GloballyHiddenMetrics.ContainsKey(game))
                windowSettings.GloballyHiddenMetrics.Add(game, new Dictionary<string, bool>());

            // Add to list if not there already.
            foreach (var key in StatsDefinitions[game].Collect.Keys)
                if (!windowSettings.GloballyHiddenMetrics[game].ContainsKey(key))
                    windowSettings.GloballyHiddenMetrics[game].Add(key, false);
        }

        IsInit = true;
    }
}