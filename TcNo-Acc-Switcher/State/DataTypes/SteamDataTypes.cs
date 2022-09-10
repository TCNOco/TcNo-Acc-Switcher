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
using Newtonsoft.Json;
using TcNo_Acc_Switcher.Shared.ContextMenu;

namespace TcNo_Acc_Switcher.State.DataTypes;

/// <summary>
/// Simple class for storing info related to Steam account, for switching and displaying.
/// </summary>
public class SteamUser
{
    [JsonIgnore] public string SteamId { get; set; }
    [JsonProperty("AccountName", Order = 0)] public string AccName { get; set; }
    [JsonProperty("PersonaName", Order = 1)] public string Name { get; set; }
    [JsonProperty("RememberPassword", Order = 2)] public string RememberPass = "1"; // Should always be 1
    [JsonProperty("mostrecent", Order = 3)] public string MostRec = "0";
    [JsonProperty("Timestamp", Order = 4)] public string LastLogin { get; set; }
    [JsonProperty("WantsOfflineMode", Order = 5)] public string OfflineMode = "0";
    [JsonIgnore] public string ImageDownloadUrl { get; set; }
    [JsonIgnore] public string ImgUrl { get; set; } = "img/QuestionMark.jpg";
    [JsonIgnore] public bool BanInfoLoaded { get; set; }
    [JsonIgnore] public bool Vac { get; set; }
    // Either Limited or Community banned status (when using Steam Web API)
    [JsonIgnore] public bool Limited { get; set; }
    [JsonIgnore] public Dictionary<string, string> AppIds { get; set; } = new();
    [JsonIgnore] public Shared.ContextMenu.MenuItem SwitchAndLaunch { get; set; } // This MUST remain null to not be added to context menu.
}

/// <summary>
/// Class for storing SteamID, VAC status and Limited status.
/// </summary>
public class VacStatus
{
    [JsonProperty(Order = 0)] public string SteamId { get; set; }
    [JsonProperty(Order = 1)] public bool Vac { get; set; }
    [JsonProperty(Order = 2)] public bool Ltd { get; set; }
}