using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.State.Classes.GameStats
{
    public class GameStatDefinition
    {
        public string UniqueId { get; set; }
        public string Indicator { get; set; }
        public string Url { get; set; }
        public string RequestCookies { get; set; }
        public Dictionary<string, string> Vars { get; set; }
        public Dictionary<string, CollectInstruction> Collect { get; set; }
    }
}
