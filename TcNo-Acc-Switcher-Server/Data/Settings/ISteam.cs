using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;

namespace TcNo_Acc_Switcher_Server.Data.Settings;

public interface ISteam
{
    void SaveSettings();
    string GetShortcutImagePath(string gameShortcutName);
    string ShortcutFolder { get; }
    List<string> InstalledGames { get; set; }
    Dictionary<string, string> AppIds { get; set; }
    bool ForgetAccountEnabled { get; set; }
    string FolderPath { get; set; }
    bool Admin { get; set; }
    bool ShowSteamId { get; set; }
    bool ShowVac { get; set; }
    bool ShowLimited { get; set; }
    bool ShowLastLogin { get; set; }
    bool ShowAccUsername { get; set; }
    bool TrayAccName { get; set; }
    int ImageExpiryTime { get; set; }
    int TrayAccNumber { get; set; }
    int OverrideState { get; set; }
    Dictionary<int, string> Shortcuts { get; set; }
    string ClosingMethod { get; set; }
    string StartingMethod { get; set; }
    bool AutoStart { get; set; }
    bool ShowShortNotes { get; set; }
    string SteamWebApiKey { get; set; }
    bool StartSilent { get; set; }
    Dictionary<string, string> CustomAccNames { get; set; }
    bool DesktopShortcut { get; set; }
    int LastAccTimestamp { get; set; }
    string LastAccSteamId { get; set; }
    bool SteamWebApiWasReset { get; set; }
    string SteamAppsListPath { get; set; }
    string SteamAppsUserCache { get; set; }
    List<string> Processes { get; set; }
    string VacCacheFile { get; set; }
    string SettingsFile { get; set; }
    string SteamImagePath { get; set; }
    string SteamImagePathHtml { get; set; }
    ObservableCollection<MenuItem> Menu { get; set; }
    void SaveShortcutOrderSteam(Dictionary<int, string> o);
    void SetClosingMethod(string method);
    void SetStartingMethod(string method);
    Task HandleShortcutActionSteam(string shortcut, string action);
    void BuildContextMenu();
    void SteamOpenUserdata();
    string StateToString(int state);

    /// <summary>
    /// Default settings for SteamSettings.json
    /// </summary>
    void ResetSettings();

    /// <summary>
    /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
    /// </summary>
    /// <returns>(Steam's path)\config\loginusers.vdf</returns>
    string LoginUsersVdf();

    /// <summary>
    /// Get Steam.exe path from SteamSettings.json
    /// </summary>
    /// <returns>Steam.exe's path string</returns>
    string Exe();

    /// <summary>
    /// Updates the ForgetAccountEnabled bool in Steam settings file
    /// </summary>
    /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
    void SetForgetAcc(bool enabled);

    /// <summary>
    /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
    /// </summary>
    string GetSteamIdCssBlock();

    /// <summary>
    /// Main function for Steam Account Switcher. Run on load.
    /// Collects accounts from Steam's loginusers.vdf
    /// Prepares images and VAC/Limited status
    /// Prepares HTML Elements string for insertion into the account switcher GUI.
    /// </summary>
    /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
    Task LoadProfiles();

    /// <summary>
    /// This relies on Steam updating loginusers.vdf. It could go out of sync assuming it's not updated reliably. There is likely a better way to do this.
    /// I am avoiding using the Steam API because it's another DLL to include, but is the next best thing - I assume.
    /// </summary>
    string GetCurrentAccountId(bool getNumericId = false);

    /// <summary>
    /// Takes loginusers.vdf and iterates through each account, loading details into output Steamuser list.
    /// </summary>
    /// <param name="loginUserPath">loginusers.vdf path</param>
    /// <returns>List of SteamUser classes, from loginusers.vdf</returns>
    Task<List<SteamUserAcc>> GetSteamUsers(string loginUserPath);

    Task DownloadSteamAppsData();
    Dictionary<string, string> LoadAppNames();
    List<string> LoadInstalledGames();
    Task ChangeUsername();

    /// <summary>
    /// Loads ban info from cache file
    /// </summary>
    /// <returns>Whether file was loaded. False if deleted ~ failed to load.</returns>
    void LoadCachedBanInfo();

    /// <summary>
    /// Saves List of VacStatus into cache file as JSON.
    /// </summary>
    void SaveVacInfo();

    string UnixTimeStampToDateTime(string stringUnixTimeStamp);

    /// <summary>
    /// Deletes outdated/invalid profile images (If they exist)
    /// Then downloads a new copy from Steam
    /// </summary>
    /// <param name="steamId"></param>
    /// <param name="noCache">Whether a new copy of XML data should be downloaded</param>
    /// <returns></returns>
    void PrepareProfile(string steamId, bool noCache);

    /// <summary>
    /// Restart Steam with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="steamId">(Optional) User's SteamID</param>
    /// <param name="ePersonaState">(Optional) Persona state for user [0: Offline, 1: Online...]</param>
    /// <param name="args">Starting arguments</param>
    Task SwapSteamAccounts(string steamId = "", int ePersonaState = -1, string args = "");

    /// <summary>
    /// Updates loginusers and registry to select an account as "most recent"
    /// </summary>
    /// <param name="selectedSteamId">Steam ID64 to switch to</param>
    /// <param name="pS">[PersonaState]0-7 custom persona state [0: Offline, 1: Online...]</param>
    Task UpdateLoginUsers(string selectedSteamId, int pS);

    /// <summary>
    /// Save updated list of Steamuser into loginusers.vdf, in vdf format.
    /// </summary>
    /// <param name="userAccounts">List of Steamuser to save into loginusers.vdf</param>
    Task SaveSteamUsersIntoVdf(List<SteamUserAcc> userAccounts);

    /// <summary>
    /// Clears images folder of contents, to re-download them on next load.
    /// </summary>
    /// <returns>Whether files were deleted or not</returns>
    Task ClearImages();

    /// <summary>
    /// Sets whether the user is invisible or not
    /// </summary>
    /// <param name="steamId">SteamID of user to update</param>
    /// <param name="ePersonaState">Persona state enum for user (0-7)</param>
    void SetPersonaState(string steamId, int ePersonaState);

    /// <summary>
    /// Returns string representation of Steam ePersonaState int
    /// </summary>
    /// <param name="ePersonaState">integer state to return string for</param>
    string PersonaStateToString(int ePersonaState);

    /// <summary>
    /// Copy settings from currently logged in account to selected game and account
    /// </summary>
    Task CopySettingsFrom(string gameId);

    Task RestoreSettingsTo(string gameId);
    Task BackupGameData(string gameId);
}