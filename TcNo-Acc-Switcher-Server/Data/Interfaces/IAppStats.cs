using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.Data;

public interface IAppStats
{
    void SaveSettings();
    void ClearStats();
    void UploadStats();
    void NewNavigation(string newPage);
    void IncrementSwitches(string platform);
    void SetAccountCount(string platform, int count);
    void IncrementGameLaunches(string platform);
    void SetGameShortcutCount(string platform, Dictionary<int, string> shortcuts);
    void GenerateTotals();
    Dictionary<string, SwitcherStat> SwitcherStats { get; set; }
}