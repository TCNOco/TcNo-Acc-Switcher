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
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Origin
    {
        private static readonly Lang Lang = Lang.Instance;
        private static Origin _instance = new();

        private static readonly object LockObj = new();
        public static Origin Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Origin();
                }
            }
            set => _instance = value;
        }

        // Variables
        private string _folderPath = "C:\\Program Files (x86)\\Origin\\";
        private bool _admin;
        private int _trayAccNumber = 3;
        private bool _forgetAccountEnabled;
        private int _imageExpiryTime = 7;
        private bool _altClose;
        private bool _desktopShortcut;
        [JsonProperty("FolderPath", Order = 1)] public static string FolderPath { get => Instance._folderPath; set => Instance._folderPath = value; }

        [JsonProperty("Origin_Admin", Order = 2)] public static bool Admin { get => Instance._admin; set => Instance._admin = value; }

        [JsonProperty("Origin_TrayAccNumber", Order = 3)] public static int TrayAccNumber { get => Instance._trayAccNumber; set => Instance._trayAccNumber = value; }

        [JsonProperty("ForgetAccountEnabled", Order = 4)] public static bool ForgetAccountEnabled { get => Instance._forgetAccountEnabled; set => Instance._forgetAccountEnabled = value; }

        [JsonProperty("ImageExpiryTime", Order = 5)] public static int ImageExpiryTime { get => Instance._imageExpiryTime; set => Instance._imageExpiryTime = value; }

        [JsonProperty("AltClose", Order = 6)] public static bool AltClose { get => Instance._altClose; set => Instance._altClose = value; }

        [JsonIgnore] public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }

        // Constants
        [JsonIgnore] public static readonly string SettingsFile = "OriginSettings.json";
        /*
        [JsonIgnore] public string OriginImagePath = "wwwroot/img/profiles/origin/";
        [JsonIgnore] public string OriginImagePathHtml = "img/profiles/origin/";
        */
        [JsonIgnore] public static readonly string Processes = "Origin";
        [JsonIgnore] public static readonly string ContextMenuJson = $@"[
				{{""{Lang["Context_SwapTo"]}"": ""swapTo(-1, event)""}},
				{{""{Lang["Context_ChangeName"]}"": ""showModal('changeUsername')""}},
				{{""{Lang["Context_CreateShortcut"]}"": ""createShortcut()""}},
				{{""{Lang["Context_ChangeImage"]}"": ""changeImage(event)""}},
				{{""{Lang["Forget"]}"": ""forget(event)""}}
            ]";


        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public static void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Origin.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Get Origin.exe path from OriginSettings.json
        /// </summary>
        /// <returns>Origin.exe's path string</returns>
        public static string Exe() => Path.Join(FolderPath, "Origin.exe");


        #region SETTINGS

        /// <summary>
        /// Default settings for OriginSettings.json
        /// </summary>
        public static void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Origin.ResetSettings]");
            FolderPath = "C:\\Program Files (x86)\\Origin\\";
            Admin = false;
            TrayAccNumber = 3;
            DesktopShortcut = Shortcut.CheckShortcuts("Origin");
            AltClose = false;

            SaveSettings();
        }
        private static void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Origin.SetFromJObject]");
            var curSettings = j.ToObject<Origin>();
            if (curSettings == null) return;
            FolderPath = curSettings._folderPath;
            Admin = curSettings._admin;
            TrayAccNumber = curSettings._trayAccNumber;
            DesktopShortcut = Shortcut.CheckShortcuts("Origin");
            AltClose = curSettings._altClose;
        }
        public static void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, JObject.FromObject(new Origin())));
        public static void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, JObject.FromObject(Instance), mergeNewIntoOld);
        #endregion
    }
}
