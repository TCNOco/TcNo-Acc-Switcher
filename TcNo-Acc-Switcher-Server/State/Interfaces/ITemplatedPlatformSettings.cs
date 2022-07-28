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
    void LoadTemplatedPlatformSettings();
    void Save();
    void Reset();

    /// <summary>
    /// Updates the ForgetAccountEnabled bool in settings file
    /// </summary>
    /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
    void SetForgetAcc(bool enabled);

    void SetClosingMethod(string method);
    void SetStartingMethod(string method);
}