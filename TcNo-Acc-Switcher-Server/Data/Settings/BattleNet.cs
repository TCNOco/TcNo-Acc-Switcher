// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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
        [JsonProperty("FolderPath", Order = 1)] public string FolderPath { get => _instance._folderPath; set => _instance._folderPath = value; }
        
        private bool _admin;
        [JsonProperty("BattleNet_Admin", Order = 3)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        
        private int _trayAccNumber = 3;
        [JsonProperty("BattleNet_TrayAccNumber", Order = 4)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 5)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }
        
        private bool _overwatchMode = true;

        [JsonProperty("OverwatchMode", Order = 6)]
        public bool OverwatchMode { get => _instance._overwatchMode; set =>_instance._overwatchMode = value; }

        private int _imageExpiryTime = 7;
        [JsonProperty("ImageExpiryTime", Order = 7)] public int ImageExpiryTime { get => _instance._imageExpiryTime; set => _instance._imageExpiryTime = value; }



        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }
        
        private List<BattleNetSwitcherBase.BattleNetUser> _accounts = new();
        [JsonIgnore] public List<BattleNetSwitcherBase.BattleNetUser> Accounts { get => _instance._accounts; set => _instance._accounts = value; }
        
        private List<string> _ignoredAccounts = new();
        [JsonIgnore] public List<string> IgnoredAccounts { get => _instance._ignoredAccounts; set => _instance._ignoredAccounts = value; }
        
        // Constants
        [JsonIgnore] public static readonly string SettingsFile = "BattleNetSettings.json";
        [JsonIgnore] public readonly string StoredAccPath = "LoginCache\\BattleNet\\StoredAccounts.json";
        [JsonIgnore] public readonly string IgnoredAccPath = "LoginCache\\BattleNet\\IgnoredAccounts.json";
        [JsonIgnore] public readonly string ImagePath = "wwwroot\\img\\profiles\\battlenet\\";
        [JsonIgnore] public readonly string ContextMenuJson = @"[
              {""Swap to account"": ""swapTo(-1, event)""},
              {""Set BattleTag"": ""showModal('changeUsername:BattleTag')""},
              {""Delete BattleTag"": ""forgetBattleTag()""},
              {""Refetch Rank"": ""refetchRank()""},
              {""Create Desktop Shortcut..."": ""createShortcut()""},
              {""Change image"": ""changeImage(event)""},
              {""Forget"": ""forget(event)""}
            ]";


        #endregion

        #region FORGETTING_ACCOUNTS
    
        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\BattleNet.SetForgetAcc]");
            if (_forgetAccountEnabled == enabled) return; // Ignore if already set
            _forgetAccountEnabled = enabled;
            SaveSettings();
        }

        #endregion
        

        /// <summary>
        /// Get Battle.net.exe path from OriginSettings.json 
        /// </summary>
        /// <returns>Battle.net.exe's path string</returns>
        public string Exe() => FolderPath + "\\Battle.net.exe";
        

        #region SETTINGS
        /// <summary>
        /// Default settings for BattleNetSettings.json
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\BattleNet.ResetSettings]");
            _instance.FolderPath = "C:\\Program Files (x86)\\Battle.net";
            _instance.Admin = false;
            _instance.TrayAccNumber = 3;
            _instance._overwatchMode = true;
            // Should this also clear ignored accounts?
            _instance._desktopShortcut = Shortcut.CheckShortcuts("BattleNet");
            SaveSettings();
        }
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\BattleNet.SetFromJObject]");
            var curSettings = j.ToObject<BattleNet>();
            if (curSettings == null) return;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.Admin = curSettings.Admin;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;
            _instance._overwatchMode = curSettings._overwatchMode;
            _instance._desktopShortcut = Shortcut.CheckShortcuts("BattleNet");
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
        public JObject GetJObject() => JObject.FromObject(this);

        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);

        /// <summary>
        /// Load the Stored Accounts and Ignored Accounts
        /// </summary>
        public void LoadAccounts()
        {
	        Directory.CreateDirectory("LoginCache\\BattleNet");
            if (File.Exists(StoredAccPath) )
            {
                Accounts = JsonConvert.DeserializeObject<List<BattleNetSwitcherBase.BattleNetUser>>(File.ReadAllText(StoredAccPath)) ?? new List<BattleNetSwitcherBase.BattleNetUser>();
            }

            if (File.Exists(IgnoredAccPath))
            {
                IgnoredAccounts = JsonConvert.DeserializeObject<List<string>>(File.ReadAllText(IgnoredAccPath)) ?? new List<string>();
            }
        }

        /// <summary>
        /// Load the Stored Accounts and Ignored Accounts
        /// </summary>
        public void SaveAccounts()
        {
            File.WriteAllText(StoredAccPath, JsonConvert.SerializeObject(Accounts));
            File.WriteAllText(IgnoredAccPath, JsonConvert.SerializeObject(IgnoredAccounts));
        }
        
        
        #endregion
    }
}
