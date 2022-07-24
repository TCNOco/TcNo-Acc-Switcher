using System.Collections.Generic;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.Classes.GameStats;

namespace TcNo_Acc_Switcher_Server.State;

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
    Dictionary<string, Dictionary<string, GameStats.StatValueAndIcon>> GetUserStatsAllGamesMarkup(string account = "");

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