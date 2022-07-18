using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;

namespace TcNo_Acc_Switcher_Server.Data.Settings;

public interface IBasic
{
    void SaveSettings();
    string FolderPath { get; set; }
    bool Admin { get; set; }
    int TrayAccNumber { get; set; }
    bool ForgetAccountEnabled { get; set; }
    Dictionary<int, string> Shortcuts { get; set; }
    string ClosingMethod { get; set; }
    string StartingMethod { get; set; }
    bool AutoStart { get; set; }
    bool ShowShortNotes { get; set; }
    bool DesktopShortcut { get; set; }
    int LastAccTimestamp { get; set; }
    string LastAccName { get; set; }
    ObservableCollection<MenuItem> ContextMenuItems { get; set; }
    ObservableCollection<MenuItem> ContextMenuShortcutItems { get; set; }
    ObservableCollection<MenuItem> ContextMenuPlatformItems { get; set; }

    /// <summary>
    /// Updates the ForgetAccountEnabled bool in settings file
    /// </summary>
    /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
    void SetForgetAcc(bool enabled);

    /// <summary>
    /// Get exe path from BasicSettings.json
    /// </summary>
    /// <returns>exe's path string</returns>
    string Exe();

    void SaveShortcutOrder(Dictionary<int, string> o);
    void SetClosingMethod(string method);
    void SetStartingMethod(string method);
    Task OpenFolder(string folder);
    void RunPlatform(string exePath, bool admin, string args, string platName, string startingMethod = "Default");
    void RunPlatform(bool admin);
    Task RunPlatform();
    Task RunShortcut(string s, string shortcutFolder = "", bool admin = false, string platform = "");
    Task HandleShortcutAction(string shortcut, string action);

    /// <summary>
    /// </summary>
    void ResetSettings();

    /// <summary>
    /// Main function for Basic Account Switcher. Run on load.
    /// Collects accounts from cache folder
    /// Prepares HTML Elements string for insertion into the account switcher GUI.
    /// </summary>
    /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
    Task LoadProfiles();

    void LoadAccountIds();
    string GetNameFromId(string accId);

    /// <summary>
    /// Restart Basic with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="accId">(Optional) User's unique account ID</param>
    /// <param name="args">Starting arguments</param>
    void SwapBasicAccounts(string accId = "", string args = "");

    Task<string> GetCurrentAccountId();
    Task ClearCache();

    /// <summary>
    /// Expands custom environment variables.
    /// </summary>
    /// <param name="path"></param>
    /// <param name="noIncludeBasicCheck">Whether to skip initializing BasicSettings - Useful for Steam and other hardcoded platforms</param>
    /// <returns></returns>
    string ExpandEnvironmentVariables(string path, bool noIncludeBasicCheck = false);

    Task<bool> BasicAddCurrent(string accName);
    Task<string> GetUniqueId();
    Task ChangeUsername(string accId, string newName, bool reload = true);
    Dictionary<string, string> ReadAllIds(string path = null);
}