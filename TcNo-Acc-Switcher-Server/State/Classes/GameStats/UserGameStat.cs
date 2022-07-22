using System;
using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.State.Classes.GameStats
{
    /// <summary>
    /// Holds statistics result for an account
    /// </summary>
    public sealed class UserGameStat
    {
        public Dictionary<string, string> Vars = new();
        /// <summary>
        /// Statistic name:value pairs
        /// </summary>
        public Dictionary<string, string> Collected = new();
        /// <summary>
        /// Keys on this list should NOT be displayed under accounts.
        /// </summary>
        public List<string> HiddenMetrics = new();
        public DateTime LastUpdated;
    }
}
