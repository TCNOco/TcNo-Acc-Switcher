using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class BasicPlatforms
    {
        // Make this a singleton
        private static BasicPlatforms _instance;
        private static readonly object LockObj = new();
        public static BasicPlatforms Instance
        {
            get
            {
                lock (LockObj)
                {
                    if (_instance != null) return _instance;

                    _instance = new BasicPlatforms();
                    _instance.BasicPlatformsInit();
                    return _instance;
                }
            }
            set => _instance = value;
        }
        // ---------------

        private JObject _jData;
        public JToken GetPlatforms => (JObject)_instance._jData["Platforms"];
        private readonly Dictionary<string, string> _platformDict = new() {
            { "BattleNet", "Battle.Net" },
            { "Epic", "Epic Games" }
        };

        private Dictionary<string, string> _platformDictAllPossible;

        public BasicPlatforms() {}
        public void BasicPlatformsInit()
        {
            _instance._jData = GeneralFuncs.LoadSettings(Path.Join(Globals.AppDataFolder, "BasicPlatforms.json"));
            _instance._platformDictAllPossible = _instance._platformDict.ToDictionary(
                entry => entry.Key,
                entry => entry.Value);
            // Populate platform Primary Token to Full Name dictionary
            foreach (var jToken in GetPlatforms)
            {
                var x = (JProperty)jToken;
                var identifiers = GetPlatforms[x.Name]["Identifiers"].ToObject<List<string>>();
                foreach (var platformShort in identifiers)
                {
                    _instance._platformDictAllPossible.Add(platformShort, x.Name);
                }
                _instance._platformDict.Add(identifiers[0], x.Name);
            }
        }

        public JObject GetPlatformJson(string platform) => (JObject)GetPlatforms![platform];
        public bool PlatformExists(string platform) => ((JObject)GetPlatforms).ContainsKey(platform);
        public bool PlatformExistsFromShort(string id) => ((JObject)GetPlatforms).ContainsKey(PlatformFullName(id));

        // ---------------
        public void SetCurrentPlatform(string platform) => CurrentPlatform.Instance.CurrentPlatformInit(platform);
        public void SetCurrentPlatformFromShort(string id) => CurrentPlatform.Instance.CurrentPlatformInit(_instance.PlatformFullName(id));
        public Dictionary<string, string> PlatformsDict => _instance._platformDict;
        public string PlatformFullName(string id) => PlatformsDict.ContainsKey(id) ? PlatformsDict[id] : id;
        public string PrimaryIdFromPlatform(string platform) => _instance._platformDictAllPossible.FirstOrDefault(x => x.Value == platform).Key;
        public string GetExeNameFromPlatform(string platform) => Path.GetFileName((string)((JObject)GetPlatforms[platform])["ExeLocationDefault"]);
        public List<string> GetAllPrimaryIds() => _instance._platformDict.Keys.ToList();
    }

    public class CurrentPlatform
    {
        // Make this a singleton
        private static CurrentPlatform _instance = new();
        private static readonly object LockObj = new();
        public static CurrentPlatform Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance;
                }
            }
            set => _instance = value;
        }
        // ---------------
        private List<string> _platformIds;
        public List<string> ExesToEnd { get; private set; }
        public bool IsInit { get; private set; }
        public string FullName { get; private set; }
        public string DefaultExePath { get; private set; }
        public string DefaultFolderPath { get; private set; }
        public string SafeName { get; private set; }
        public string SettingsFile { get; private set; }
        public string ExeName { get; private set; }
        public Dictionary<string, string> LoginFiles { get; private set; } = new();
        public List<string> PathListToClear { get; private set; }
        public CurrentPlatform(){}
        public void CurrentPlatformInit(string platform)
        {
            if (!(BasicPlatforms.Instance.PlatformExists(platform) || BasicPlatforms.Instance.PlatformExistsFromShort(platform))) return;

            _instance.FullName = platform;
            _instance.SafeName = Globals.GetCleanFilePath(platform);
            _instance.SettingsFile = _instance.SafeName + ".json";

            _instance.DefaultExePath = Environment.ExpandEnvironmentVariables((string)BasicPlatforms.Instance.GetPlatformJson(platform)["ExeLocationDefault"]);
            _instance.DefaultFolderPath = Environment.ExpandEnvironmentVariables(Path.GetDirectoryName(_instance.DefaultExePath));
            _instance.ExeName = Path.GetFileName(_instance.DefaultExePath);

            var jPlatform = BasicPlatforms.Instance.GetPlatformJson(platform);
            _instance._platformIds = jPlatform["Identifiers"]!.Values<string>().ToList();
            _instance.ExesToEnd = jPlatform["ExesToEnd"]!.Values<string>().ToList();
            _instance.PathListToClear = jPlatform["PathListToClear"]!.Values<string>().ToList();
            _instance.LoginFiles.Clear();
            foreach (var (k,v) in jPlatform["LoginFiles"].ToObject<Dictionary<string, string>>())
            {
                _instance.LoginFiles.Add(Environment.ExpandEnvironmentVariables(k), v);
            }

            // Variables that may not be set:
            if (jPlatform.ContainsKey("UniqueIdFile")) _instance.UniqueIdFile = (string)jPlatform["UniqueIdFile"];
            if (jPlatform.ContainsKey("UniqueIdFolder")) _instance.UniqueIdFolder = (string)jPlatform["UniqueIdFolder"];
            if (jPlatform.ContainsKey("UniqueIdRegex")) _instance.UniqueIdRegex = Globals.ExpandRegex((string)jPlatform["UniqueIdRegex"]);
            if (jPlatform.ContainsKey("UniqueIdMethod")) _instance.UniqueIdMethod = Globals.ExpandRegex((string)jPlatform["UniqueIdMethod"]);

            // Either load existing, or safe default settings for platform
            if (File.Exists(Path.Join(Globals.UserDataFolder, _instance.SettingsFile))) Settings.Basic.Instance.LoadFromFile();
            else Settings.Basic.Instance.SaveSettings();

            _instance.IsInit = true;
        }
        // Variables that may not be set:
        public string LoginFileFromValue(string val) =>Environment.ExpandEnvironmentVariables(_instance.LoginFiles.FirstOrDefault(x => x.Value == val).Key);
        public string GetUniqueFilePath() => _instance.LoginFileFromValue(_instance.UniqueIdFile);
        public string UniqueIdFile { get; private set; } = "";
        public string UniqueIdFolder { get; private set; } = "";
        public string UniqueIdRegex { get; private set; } = "";
        public string UniqueIdMethod { get; private set; } = "";
        public string PrimaryId => _platformIds[0];
        public string IdsJsonPath => $"LoginCache\\{_instance.SafeName}\\ids.json";
        public string AccountLoginCachePath(string acc) => $"LoginCache\\{_instance.SafeName}\\{acc}\\";
    }
}
