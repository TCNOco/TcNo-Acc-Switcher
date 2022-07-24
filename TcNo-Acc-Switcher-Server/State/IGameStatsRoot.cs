using System.Collections.Generic;
using TcNo_Acc_Switcher_Server.State.Classes.GameStats;

namespace TcNo_Acc_Switcher_Server.State;

public interface IGameStatsRoot
{
    bool IsInit { get; set; }
    Dictionary<string, GameStatDefinition> StatsDefinitions { get; set; }
    Dictionary<string, List<string>> PlatformCompatibilities { get; set; }

    /// <summary>
    /// List of all games with stats
    /// </summary>
    List<string> GameList { get; set; }

    bool GameExists(string game);
}