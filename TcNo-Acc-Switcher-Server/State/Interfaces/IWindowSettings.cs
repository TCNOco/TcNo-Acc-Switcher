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
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Drawing;
using TcNo_Acc_Switcher_Server.State.Classes;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface IWindowSettings
{
    event PropertyChangedEventHandler PropertyChanged;
    string Language { get; set; }
    bool Rtl { get; set; }
    bool StreamerModeEnabled { get; set; }
    int ServerPort { get; set; }
    Point WindowSize { get; set; }
    bool AllowTransparency { get; set; }
    string Version { get; set; }
    List<object> DisabledPlatforms { get; }
    bool TrayMinimizeNotExit { get; set; }
    bool ShownMinimizedNotification { get; set; }
    bool StartCentered { get; set; }
    string ActiveTheme { get; set; }
    string ActiveBrowser { get; set; }
    string Background { get; set; }
    List<string> EnabledBasicPlatforms { get; }
    bool CollectStats { get; set; }
    bool ShareAnonymousStats { get; set; }
    bool MinimizeOnSwitch { get; set; }
    bool DiscordRpcEnabled { get; set; }
    bool DiscordRpcShareTotalSwitches { get; set; }
    string PasswordHash { get; set; }

    /// <summary>
    /// For BasicStats // Game statistics collection and showing
    /// Keys for metrics on this list are not shown for any account.
    /// List of all games:[Settings:Hidden metric] metric keys.
    /// </summary>
    Dictionary<string, Dictionary<string, bool>> GloballyHiddenMetrics { get; set; }

    bool AlwaysAdmin { get; set; }
    ObservableCollection<PlatformItem> Platforms { get; set; }

    /// <summary>
    /// Add hard-coded platforms to the list, like Steam.
    /// </summary>
    void AddStaticPlatforms();

    void SavePlatformOrder(string jsonString);
    void Save();

    /// <summary>
    /// Get platform details from an identifier, or the name.
    /// </summary>
    PlatformItem GetPlatform(string nameOrId);
}