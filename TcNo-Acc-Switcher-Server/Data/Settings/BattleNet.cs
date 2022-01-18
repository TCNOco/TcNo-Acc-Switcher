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

// Special thanks to iR3turnZ for contributing to this platform's account switcher
// iR3turnZ: https://github.com/HoeblingerDaniel

using System.Collections.Generic;
using System.IO;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.BattleNet;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class BattleNet
    {
        private static readonly Lang Lang = Lang.Instance;

        private static BattleNet _instance = new();

        private static readonly object LockObj = new();
        public static BattleNet Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new BattleNet();
                }
            }
            set => _instance = value;
        }

        #region VARIABLES

        private string _folderPath = "C:\\Program Files (x86)\\Battle.net";
        private int _trayAccNumber = 3;
        private bool _admin;
        private bool _forgetAccountEnabled;
        private bool _overwatchMode = true;
        private int _imageExpiryTime = 7;
        private bool _altClose;
        private bool _desktopShortcut;
        private List<BattleNetSwitcherBase.BattleNetUser> _accounts = new();
        private List<string> _ignoredAccounts = new();

        [JsonProperty("FolderPath", Order = 1)] public static string FolderPath { get => Instance._folderPath; set => Instance._folderPath = value; }

        [JsonProperty("BattleNet_Admin", Order = 3)] public static bool Admin { get => Instance._admin; set => Instance._admin = value; }

        [JsonProperty("BattleNet_TrayAccNumber", Order = 4)] public static int TrayAccNumber { get => Instance._trayAccNumber; set => Instance._trayAccNumber = value; }

        [JsonProperty("ForgetAccountEnabled", Order = 5)] public static bool ForgetAccountEnabled { get => Instance._forgetAccountEnabled; set => Instance._forgetAccountEnabled = value; }

        [JsonProperty("OverwatchMode", Order = 6)] public static bool OverwatchMode { get => Instance._overwatchMode; set => Instance._overwatchMode = value; }

        [JsonProperty("ImageExpiryTime", Order = 7)] public static int ImageExpiryTime { get => Instance._imageExpiryTime; set => Instance._imageExpiryTime = value; }

        [JsonProperty("AltClose", Order = 8)] public static bool AltClose { get => Instance._altClose; set => Instance._altClose = value; }

        [JsonIgnore] public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }

        [JsonIgnore] public static List<BattleNetSwitcherBase.BattleNetUser> Accounts { get => Instance._accounts; set => Instance._accounts = value; }

        [JsonIgnore] public static List<string> IgnoredAccounts { get => Instance._ignoredAccounts; set => Instance._ignoredAccounts = value; }

        // Constants
        [JsonIgnore] public static readonly string SettingsFile = "BattleNetSettings.json";
        [JsonIgnore] public static readonly string Processes = "Battle.net";
        [JsonIgnore] public static readonly string StoredAccPath = "LoginCache\\BattleNet\\StoredAccounts.json";
        [JsonIgnore] public static readonly string IgnoredAccPath = "LoginCache\\BattleNet\\IgnoredAccounts.json";
        [JsonIgnore] public static readonly string ImagePath = "wwwroot\\img\\profiles\\battlenet\\";
        [JsonIgnore] public static readonly string ContextMenuJson = $@"[
              {{""{Lang["Context_SwapTo"]}"": ""swapTo(-1, event)""}},
              {{""{Lang["Context_BNet_SetBTag"]}"": ""showModal('changeUsername:BattleTag')""}},
              {{""{Lang["Context_BNet_DelBTag"]}"": ""forgetBattleTag()""}},
              {{""{Lang["Context_BNet_GetRAnk"]}"": ""refetchRank()""}},
              {{""{Lang["Context_CreateShortcut"]}"": ""createShortcut()""}},
              {{""{Lang["Context_ChangeImage"]}"": ""changeImage(event)""}},
              {{""{Lang["Forget"]}"": ""forget(event)""}}
            ]";

        #endregion

        #region FORGETTING_ACCOUNTS

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public static void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\BattleNet.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        #endregion


        /// <summary>
        /// Get Battle.net.exe path from OriginSettings.json
        /// </summary>
        /// <returns>Battle.net.exe's path string</returns>
        public static string Exe() => FolderPath + "\\Battle.net.exe";


        #region SETTINGS

        /// <summary>
        /// Default settings for BattleNetSettings.json
        /// </summary>
        public static void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\BattleNet.ResetSettings]");
            FolderPath = "C:\\Program Files (x86)\\Battle.net";
            Admin = false;
            TrayAccNumber = 3;
            OverwatchMode = true;
            // Should this also clear ignored accounts?
            DesktopShortcut = Shortcut.CheckShortcuts("BattleNet");
            AltClose = false;

            SaveSettings();
        }
        public static void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\BattleNet.SetFromJObject]");
            var curSettings = j.ToObject<BattleNet>();
            if (curSettings == null) return;
            FolderPath = curSettings._folderPath;
            Admin = curSettings._admin;
            TrayAccNumber = curSettings._trayAccNumber;
            OverwatchMode = curSettings._overwatchMode;
            DesktopShortcut = Shortcut.CheckShortcuts("BattleNet");
            AltClose = curSettings._altClose;
        }
        public static void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, JObject.FromObject(new BattleNet())));

        public static void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, JObject.FromObject(Instance), mergeNewIntoOld);

        /// <summary>
        /// Load the Stored Accounts and Ignored Accounts
        /// </summary>
        public static void LoadAccounts()
        {
            _ = Directory.CreateDirectory("LoginCache\\BattleNet");
            if (File.Exists(StoredAccPath))
            {
                Accounts = JsonConvert.DeserializeObject<List<BattleNetSwitcherBase.BattleNetUser>>(Globals.ReadAllText(StoredAccPath)) ?? new List<BattleNetSwitcherBase.BattleNetUser>();
            }

            if (File.Exists(IgnoredAccPath))
            {
                IgnoredAccounts = JsonConvert.DeserializeObject<List<string>>(Globals.ReadAllText(IgnoredAccPath)) ?? new List<string>();
            }
        }

        /// <summary>
        /// Load the Stored Accounts and Ignored Accounts
        /// </summary>
        public static void SaveAccounts()
        {
            File.WriteAllText(StoredAccPath, JsonConvert.SerializeObject(Accounts));
            File.WriteAllText(IgnoredAccPath, JsonConvert.SerializeObject(IgnoredAccounts));
        }

        #endregion
    }
}
