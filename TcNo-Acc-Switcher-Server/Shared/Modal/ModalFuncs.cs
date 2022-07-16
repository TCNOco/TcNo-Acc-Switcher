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

using System.IO;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Shared.Modal
{
    public class ModalFuncs
    {
        public static void UpdatePlatformFolder()
        {
            var path = ModalData.PathPicker.LastPath;

            Globals.DebugWriteLine($@"[ModalFuncs.UpdatePlatformFolder] file={AppData.CurrentSwitcher}, path={path}");
            var settingsFile = AppData.CurrentSwitcher == "Steam"
                ? SteamSettings.SettingsFile
                : CurrentPlatform.SettingsFile;

            var settings = GeneralFuncs.LoadSettings(settingsFile);
            settings["FolderPath"] = path;
            GeneralFuncs.SaveSettings(settingsFile, settings);
            if (!Globals.IsFolder(path))
                path = Path.GetDirectoryName(path); // Remove .exe
            if (!string.IsNullOrWhiteSpace(path) && path.EndsWith(".exe"))
                path = Path.GetDirectoryName(path) ?? string.Join("\\", path.Split("\\")[..^1]);

            if (AppData.CurrentSwitcher == "Steam")
                SteamSettings.FolderPath = path;
            else
                BasicSettings.FolderPath = path;
        }
    }
}
