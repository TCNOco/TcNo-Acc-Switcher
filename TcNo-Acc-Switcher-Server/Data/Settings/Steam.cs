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
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Steam
    {
        private static readonly Lang Lang = Lang.Instance;
        private static Steam _instance = new();
        private static readonly object LockObj = new();
        public static Steam Instance
        {
            get
            {
                lock (LockObj)
                {
                    // Load settings if have changed, or not set
                    if (_instance is { _currentlyModifying: true }) return _instance;
                    if (_instance._lastHash != "") return _instance;

                    _instance = new Steam { _currentlyModifying = true };

                    if (File.Exists(SettingsFile))
                    {
                        if (File.Exists(SettingsFile)) JsonConvert.PopulateObject(File.ReadAllText(SettingsFile), _instance);
                        if (_instance == null)
                        {
                            _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedLoadSettings"]);
                            if (File.Exists(SettingsFile))
                                Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
                            _instance = new Steam { _currentlyModifying = true };
                        }
                        _instance._lastHash = Globals.GetFileMd5(SettingsFile);
                        if (_instance._folderPath.EndsWith(".exe"))
                            _instance._folderPath = Path.GetDirectoryName(_instance._folderPath) ?? string.Join("\\", _instance._folderPath.Split("\\")[..^1]);
                    }
                    else
                    {
                        SaveSettings();
                    }
                    LoadBasicCompat(); // Add missing features in templated platforms system.

                    //// Forces lazy values to be instantiated
                    //_ = InstalledGames.Value;
                    //_ = AppIds.Value;

                    BuildContextMenu();

                    _instance._currentlyModifying = false;

                    return _instance;
                }
            }
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }

        private string _lastHash = "";
        private bool _currentlyModifying;

        public static void SaveSettings() => Globals.SaveJsonFile(SettingsFile, Instance);


        // Constants



    }
}
