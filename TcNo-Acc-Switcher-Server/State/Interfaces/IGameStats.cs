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
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.Classes.GameStats;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface IGameStats
{
    /// <summary>
    /// Dictionary of Game Name:Available user game statistics
    /// </summary>
    Dictionary<string, GameStatSaved> SavedStats { get; set; }

    List<string> GetAvailableGames { get; }

    /// <summary>
    /// Loads games and stats for requested platform
    /// </summary>
    /// <returns>If successfully loaded platform</returns>
    Task<bool> SetCurrentPlatform(string platform);

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
    Dictionary<string, Dictionary<string, StatValueAndIcon>> GetUserStatsAllGamesMarkup(string account = "");

    /// <summary>
    /// Gets list of all metric names to collect, as well as whether each is hidden or not, and the text to display in the UI checkbox.
    /// Example: "AP":"Arena Points"
    /// </summary>
    //public static Dictionary<string, Tuple<bool, string>> GetAllMetrics(string game)
    Dictionary<string, string> GetAllMetrics(string game);

    bool PlatformHasAnyGames(string platform);

    /// <summary>
    /// Get longer game name from it's short unique ID.
    /// </summary>
    string GetGameNameFromId(string id);

    /// <summary>
    /// Get short unique ID from game name.
    /// </summary>
    string GetGameIdFromName(string name);

    Task RefreshAllAccounts(string game, string platform = "");
    List<string> PlatformCompatibilitiesWithStats(string platform);

    /// <summary>
    /// Returns a string with all the statistics available for the specified account
    /// </summary>
    string GetSavedStatsString(string accountId, string sep, bool isBasic = false);
}