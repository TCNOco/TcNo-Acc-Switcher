using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class BattleNet
    {
        private static BattleNet _instance = new BattleNet();

        public BattleNet(){}
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

        // Variables
        private string _folderPath = "C:\\Program Files (x86)\\Battle.net";
        [JsonProperty("FolderPath", Order = 1)] public string FolderPath { get => _instance._folderPath; set => _instance._folderPath = value; }
        private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("WindowSize", Order = 2)] public Point WindowSize { get => _instance._windowSize; set => _instance._windowSize = value; }
        private bool _admin;
        [JsonProperty("Origin_Admin", Order = 3)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Origin_TrayAccNumber", Order = 4)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 5)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }
        private Dictionary<string, string> _bTags = new Dictionary<string, string>();
        [JsonProperty("BTags", Order = 6)] public Dictionary<string, string> BTags { get => _instance._bTags; set => _instance._bTags = value; }
        
        
        // Constants
        [JsonIgnore] public string SettingsFile = "BattleNetSettings.json";
        [JsonIgnore] public string BattleNetImagePath = "wwwroot/img/profiles/battlenet/";
        [JsonIgnore] public string BattleNetImagePathHtml = "img/profiles/battlenet/";
        [JsonIgnore] public string ContextMenuJson = @"[
              {""Swap to account"": ""SwapTo(-1, event)""},
              {""Set BattleTag"": ""ShowModal('setBTag')""},
              {""Delete BattleTag"": ""DeleteBTag()""},
            ]";
        

        /// <summary>
        /// Default settings for BattleNetSettings.json
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\BattleNet.ResetSettings]");
            _instance.FolderPath = "C:\\Program Files (x86)\\Battle.net";
            _instance.WindowSize = new Point() { X = 800, Y = 450 };
            _instance.Admin = false;
            _instance.TrayAccNumber = 3;
            _instance.BTags = new Dictionary<string, string>();
            SaveSettings();
        }
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\BattleNet.SetFromJObject]");
            var curSettings = j.ToObject<BattleNet>();
            if (curSettings == null) return;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.WindowSize = curSettings.WindowSize;
            _instance.Admin = curSettings.Admin;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;
            _instance._bTags = JsonConvert.DeserializeObject<Dictionary<string,string>>(j.SelectToken("BTags").ToString());
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
        public JObject GetJObject() => JObject.FromObject(this);
        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);

        /// <summary>
        /// Get Origin.exe path from OriginSettings.json 
        /// </summary>
        /// <returns>Origin.exe's path string</returns>
        public string Exe() =>  FolderPath + "\\Battle.net.exe";
    }
}
