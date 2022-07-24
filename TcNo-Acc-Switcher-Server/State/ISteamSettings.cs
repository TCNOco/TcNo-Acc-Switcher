using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.State;

public interface ISteamSettings
{
    bool ForgetAccountEnabled { get; set; }
    string FolderPath { get; set; }
    bool Admin { get; set; }
    bool ShowSteamId { get; set; }
    bool ShowVac { get; set; }
    bool ShowLimited { get; set; }
    bool ShowLastLogin { get; set; }
    bool ShowAccUsername { get; set; }
    bool TrayAccountName { get; set; }
    int ImageExpiryTime { get; set; }
    int TrayAccNumber { get; set; }
    int OverrideState { get; set; }
    string SteamWebApiKey { get; set; }
    Dictionary<int, string> Shortcuts { get; set; }
    string ClosingMethod { get; set; }
    string StartingMethod { get; set; }
    bool AutoStart { get; set; }
    bool ShowShortNotes { get; set; }
    bool StartSilent { get; set; }
    Dictionary<string, string> CustomAccountNames { get; set; }

    /// <summary>
    /// Get Steam.exe path from SteamSettings.json
    /// </summary>
    /// <returns>Steam.exe's path string</returns>
    string Exe { get; }

    void Save();
    void Reset();
    void SaveShortcutOrderSteam(Dictionary<int, string> o);
    void SetClosingMethod(string method);
    void SetStartingMethod(string method);

    /// <summary>
    /// Updates the ForgetAccountEnabled bool in Steam settings file
    /// </summary>
    /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
    void SetForgetAcc(bool enabled);
}