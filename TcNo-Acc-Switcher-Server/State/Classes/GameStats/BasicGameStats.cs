using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.State.Classes.GameStats
{

    /// <summary>
    /// Stores number of accounts and list of hidden metrics for each game
    /// </summary>
    public class BasicGameStats
    {
        public int NumAccounts = 0;
        public Dictionary<string, int> Metrics = new();
    }
}
