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

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

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
    string LoginUsersVdf { get; set; }
    string LibraryVdf { get; set; }
    string SteamImagePath { get; init; }
    string SteamImagePathHtml { get; init; }
    List<string> Processes { get; init; }
    string VacCacheFile { get; init; }

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