using System.Collections.Generic;
using TcNo_Acc_Switcher_Server.State.Classes.Steam;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State;

public interface ISteamState
{
    List<SteamUser> SteamUsers { get; set; }
    bool SteamLoadingProfiles { get; set; }
    SteamContextMenu ContextMenu { get; set; }
    bool DesktopShortcut { get; set; }
    int LastAccTimestamp { get; set; }
    string LastAccSteamId { get; set; }
    bool SteamWebApiWasReset { get; set; }

    /// <summary>
    /// Loads ban info from cache file
    /// </summary>
    /// <returns>Whether file was loaded. False if deleted ~ failed to load.</returns>
    void LoadCachedBanInfo();

    /// <summary>
    /// Deletes outdated/invalid profile images (If they exist)
    /// Then downloads a new copy from Steam
    /// </summary>
    /// <param name="steamId"></param>
    /// <param name="noCache">Whether a new copy of XML data should be downloaded</param>
    /// <returns></returns>
    void PrepareProfile(string steamId, bool noCache);

    /// <summary>
    /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
    /// </summary>
    /// <returns>(Steam's path)\config\loginusers.vdf</returns>
    string CheckLoginUsersVdfPath();

    bool SteamSettingsValid();

    /// <summary>
    /// Takes loginusers.vdf and iterates through each account, loading details into output Steamuser list.
    /// </summary>
    /// <param name="loginUserPath">loginusers.vdf path</param>
    /// <returns>List of SteamUser classes, from loginusers.vdf</returns>
    List<SteamUser> GetSteamUsers(string loginUserPath);

    /// <summary>
    /// Saves List of VacStatus into cache file as JSON.
    /// </summary>
    void SaveVacInfo();

    void HandleShortcutActionSteam(string shortcut, string action);
    void RunSteam(bool admin, string args);
    void LoadNotes();
}