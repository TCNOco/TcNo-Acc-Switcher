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
using System.Linq;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.Classes.GameStats;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class GameStats : IGameStats
{
    private readonly IAppState _appState;
    private readonly IGameStatsRoot _gameStatsRoot;
    private readonly ILang _lang;
    private readonly IToasts _toasts;

    /// <summary>
    /// Read GameStats.json and collect game definitions, as well as platform-game relations.
    /// </summary>
    public GameStats(IAppState appState, IGameStatsRoot gameStatsRoot, ILang lang, IToasts toasts)
    {
        _appState = appState;
        _gameStatsRoot = gameStatsRoot;
        _lang = lang;
        _toasts = toasts;
    }

    // TEMP NOTES:
    // - PlatformGames is now GameStatsRoot.PlatformCompatibilities



    #region Load games


    private readonly List<string> _currentlyRefreshing = new();
    public async Task RefreshSelectedAccount(string game)
    {
        if (_currentlyRefreshing.Contains(game)) return;
        _currentlyRefreshing.Add(game);
        await SavedStats[game].LoadStatsFromWeb(_appState.Switcher.SelectedAccountId, _appState.Switcher.CurrentSwitcher);
        SavedStats[game].SaveStats();
        _currentlyRefreshing.Remove(game);
    }

    /// <summary>
    /// Dictionary of Game Name:Available user game statistics
    /// </summary>
    public Dictionary<string, GameStatSaved> SavedStats { get; set; } = new();


    /// <summary>
    /// Loads games and stats for requested platform
    /// </summary>
    /// <returns>If successfully loaded platform</returns>
    public async Task<bool> SetCurrentPlatform(string platform)
    {
        if (!_gameStatsRoot.IsInit) return false;

        _appState.Switcher.CurrentSwitcher = platform;
        if (!_gameStatsRoot.PlatformCompatibilities.ContainsKey(platform)) return false;

        // TODO: Verify this works as intended when more games are added.
        SavedStats = new Dictionary<string, GameStatSaved>();
        foreach (var game in _gameStatsRoot.PlatformCompatibilities[platform])
        {
            var gs = new GameStatSaved(_lang, _gameStatsRoot, _appState, _toasts);
            await gs.SetGameStat(game);
            SavedStats.Add(game, gs);
        }

        return true;
    }
    #endregion

    public UserGameStat GetUserGameStat(string game, string accountId) =>
        SavedStats[game].CachedStats.ContainsKey(accountId) ? SavedStats[game].CachedStats[accountId] : null;

    /// <summary>
    /// Get list of games for current platform - That have at least 1 account for settings.
    /// </summary>
    public List<string> GetAllCurrentlyEnabledGames()
    {
        var games = new HashSet<string>();
        foreach (var gameStat in SavedStats)
        {
            // Foreach account and game pair in SavedStats
            // Add game name to list
            if (gameStat.Value.CachedStats.Any()) games.Add(gameStat.Value.Game);
        }

        return games.ToList();
    }

    /// <summary>
    /// Get and return icon html markup for specific game
    /// </summary>
    public string GetIcon(string game, string statName) => SavedStats[game].ToCollect[statName].Icon;

    // List of possible games on X platform
    public List<string> GetAvailableGames => _gameStatsRoot.PlatformCompatibilities.ContainsKey(_appState.Switcher.CurrentSwitcher) ? _gameStatsRoot.PlatformCompatibilities[_appState.Switcher.CurrentSwitcher] : new List<string>();

    /// <summary>
    /// Returns list Dictionary of Game Names:[Dictionary of statistic names and StatValueAndIcon (values and indicator text for HTML)]
    /// </summary>
    public Dictionary<string, Dictionary<string, StatValueAndIcon>> GetUserStatsAllGamesMarkup(string account = "")
    {
        if (account == "") account = _appState.Switcher.SelectedAccountId;

        var returnDict = new Dictionary<string, Dictionary<string, StatValueAndIcon>>();
        // Foreach available game
        foreach (var availableGame in GetAvailableGames)
        {
            if (!SavedStats.ContainsKey(availableGame)) continue;

            // That has the requested account
            if (!SavedStats[availableGame].CachedStats.ContainsKey(account)) continue;
            var gameIndicator = SavedStats[availableGame].Indicator;
            //var gameUniqueId = SavedStats[availableGame].UniqueId;

            var statValueIconDict = new Dictionary<string, StatValueAndIcon>();

            // Add icon or identifier to stat pair for displaying
            foreach (var (statName, statValue) in SavedStats[availableGame].CachedStats[account].Collected)
            {
                if (SavedStats[availableGame].CachedStats[account].HiddenMetrics.Contains(statName)) continue;

                // Foreach stat
                // Check if has icon, otherwise use just indicator string
                var indicatorMarkup = GetIcon(availableGame, statName);
                if (string.IsNullOrEmpty(indicatorMarkup) && !string.IsNullOrEmpty(gameIndicator) && SavedStats[availableGame].ToCollect[statName].SpecialType is not "ImageDownload")
                    indicatorMarkup = $"<sup>{gameIndicator}</sup>";

                statValueIconDict.Add(statName, new StatValueAndIcon
                {
                    StatValue = statValue,
                    IndicatorMarkup = indicatorMarkup
                });
            }

            //returnDict[gameUniqueId] = statValueIconDict;
            returnDict[availableGame] = statValueIconDict;
        }

        return returnDict;
    }

    /// <summary>
    /// Gets list of all metric names to collect, as well as whether each is hidden or not, and the text to display in the UI checkbox.
    /// Example: "AP":"Arena Points"
    /// </summary>
    //public static Dictionary<string, Tuple<bool, string>> GetAllMetrics(string game)
    public Dictionary<string, string> GetAllMetrics(string game)
    {
        //// Get hidden metrics for this game
        //var hiddenMetrics = new List<string>();
        //if (WindowSettings.GloballyHiddenMetrics.ContainsKey(game))
        //    hiddenMetrics = WindowSettings.GloballyHiddenMetrics[game];

        // Get list of all metrics and add to list.
        //var allMetrics = new Dictionary<string, Tuple<bool, string>>();
        var allMetrics = new Dictionary<string, string>();
        foreach (var (key, ci) in SavedStats[game].ToCollect)
        {
            //allMetrics.Add(key, new Tuple<bool, string>(hiddenMetrics.Contains(key), ci.ToggleText));
            allMetrics.Add(key, ci.ToggleText);
        }

        return allMetrics;
    }

    public bool PlatformHasAnyGames(string platform) => platform is not null && (_gameStatsRoot.PlatformCompatibilities.ContainsKey(platform) && _gameStatsRoot.PlatformCompatibilities[platform].Count > 0);
    //public JObject GetGame(string game) => (JObject)StatsDefinitions![game];


    /// <summary>
    /// Get longer game name from it's short unique ID.
    /// </summary>
    public string GetGameNameFromId(string id) => SavedStats.FirstOrDefault(x => x.Value.UniqueId.Equals(id, StringComparison.OrdinalIgnoreCase)).Key;
    /// <summary>
    /// Get short unique ID from game name.
    /// </summary>
    public string GetGameIdFromName(string name) => SavedStats.FirstOrDefault(x => x.Key.Equals(name, StringComparison.OrdinalIgnoreCase)).Value.UniqueId;

    public async Task RefreshAllAccounts(string game, string platform = "")
    {
        foreach (var id in SavedStats[game].CachedStats.Keys)
        {
            await SavedStats[game].LoadStatsFromWeb(id, platform);
        }

        if (game != "")
            SavedStats[game].SaveStats();
    }

    public List<string> PlatformCompatibilitiesWithStats(string platform) => _gameStatsRoot.PlatformCompatibilities.ContainsKey(platform) ? GetAvailableGames : new List<string>();


    /// <summary>
    /// Returns a string with all the statistics available for the specified account
    /// </summary>
    public string GetSavedStatsString(string accountId, string sep, bool isBasic = false)
    {
        var outputString = "";
        // Foreach game in platform
        var oneAdded = false;
        foreach (var gs in SavedStats)
        {
            // Check to see if it contains the requested accountID
            var cachedStats = gs.Value.CachedStats;
            if (!cachedStats.ContainsKey(accountId)) continue;

            if (!oneAdded) outputString += $"{gs.Key}:,";
            else
            {
                if (isBasic)
                    outputString += $"{Environment.NewLine},{gs.Key}:,";
                else
                    outputString += $"{Environment.NewLine},,,,,,{gs.Key}:,";
                oneAdded = false;
            }

            // Add each stat from account to the string, starting with the game name.
            var collectedStats = cachedStats[accountId].Collected;
            foreach (var stat in collectedStats)
            {
                if (oneAdded)
                {
                    if (isBasic)
                        outputString += $"{Environment.NewLine},, {stat.Key}{sep}{stat.Value.Replace(sep, " ")}";
                    else
                        outputString +=
                            $"{Environment.NewLine},,,,,,, {stat.Key}{sep}{stat.Value.Replace(sep, " ")}";
                }
                else
                {
                    outputString += $"{stat.Key}{sep}{stat.Value.Replace(sep, " ")}";
                    oneAdded = true;
                }
            }
        }

        return outputString;
    }
}