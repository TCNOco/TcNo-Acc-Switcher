using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.State;

public interface ITemplatedPlatformSettings
{
    bool Admin { get; set; }
    int TrayAccNumber { get; set; }
    bool ForgetAccountEnabled { get; set; }
    Dictionary<int, string> Shortcuts { get; set; }
    bool AutoStart { get; set; }
    bool ShowShortNotes { get; set; }
    bool DesktopShortcut { get; set; }
    int LastAccTimestamp { get; set; }
    string LastAccName { get; set; }
    string Exe { get; set; }
    string ClosingMethod { get; set; }
    string StartingMethod { get; set; }
    string FolderPath { get; set; }
    void Save();
    void Reset();

    /// <summary>
    /// Updates the ForgetAccountEnabled bool in settings file
    /// </summary>
    /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
    void SetForgetAcc(bool enabled);

    void SaveShortcutOrder(Dictionary<int, string> o);
    void SetClosingMethod(string method);
    void SetStartingMethod(string method);
}