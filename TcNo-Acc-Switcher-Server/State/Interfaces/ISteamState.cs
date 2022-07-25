// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System.Collections.Generic;
using TcNo_Acc_Switcher_Server.State.Classes.Steam;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface ISteamState
{
    List<SteamUser> SteamUsers { get; set; }
    bool SteamLoadingProfiles { get; set; }
    SteamContextMenu ContextMenu { get; set; }
    bool DesktopShortcut { get; set; }
    int LastAccTimestamp { get; set; }
    string LastAccSteamId { get; set; }
    bool SteamWebApiWasReset { get; set; }
    void LoadSteamState(ISteamFuncs steamFuncs);
    string GetName(SteamUser su);

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