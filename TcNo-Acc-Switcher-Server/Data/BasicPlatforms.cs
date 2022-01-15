using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
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
        private Dictionary<string, string> _platformDict = new();

        private readonly Dictionary<string, string> _platformDictAllPossible = new()
        {
            { "BattleNet", "Battle.Net" }
        };

        public BasicPlatforms() {}
        public void BasicPlatformsInit()
        {
            _instance._jData = GeneralFuncs.LoadSettings(Path.Join(Globals.AppDataFolder, "BasicPlatforms.json"));
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
        public void SetCurrentPlatform(string platform)
        {
            CurrentPlatform.Instance = new CurrentPlatform();
            CurrentPlatform.Instance.CurrentPlatformInit(platform);
        }
        public void SetCurrentPlatformFromShort(string id)
        {
            CurrentPlatform.Instance = new CurrentPlatform();
            CurrentPlatform.Instance.CurrentPlatformInit(_instance.PlatformFullName(id));
        }
        public Dictionary<string, string> PlatformsDict => _instance._platformDict;

        private Dictionary<string, string> InactivePlatforms()
        {
            // Create local copy of platforms dict:
            var platforms = _instance._platformDict.ToDictionary(
                entry => entry.Key,
                entry => entry.Value);

            foreach (var enabledPlat in AppSettings.Instance.EnabledBasicPlatforms)
                platforms.Remove(enabledPlat);

            return platforms;
        }

        public List<KeyValuePair<string, string>> InactivePlatformsSorted()
        {
            var inactive = InactivePlatforms().ToList();
            inactive.Sort((p1, p2) => string.Compare(p1.Key, p2.Key, StringComparison.Ordinal));
            return inactive;
        }
        public string PlatformFullName(string id) => PlatformsDict.ContainsKey(id) ? PlatformsDict[id] : id;
        public string PlatformSafeName(string id) =>
            PlatformsDict.ContainsKey(id) ? Globals.GetCleanFilePath(PlatformsDict[id]) : id;
        public string PrimaryIdFromPlatform(string platform) => _instance._platformDictAllPossible.FirstOrDefault(x => x.Value == platform).Key;
        public string GetExeNameFromPlatform(string platform) => Path.GetFileName((string)((JObject)GetPlatforms[platform])["ExeLocationDefault"]);
        public List<string> GetAllPrimaryIds() => _instance._platformDict.Keys.ToList();
    }

    public class CurrentPlatform
    {
        public CurrentPlatform() { }
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
        public string ExeExtraArgs { get; private set; }
        public string DefaultFolderPath { get; private set; }
        public string SafeName { get; private set; }
        public string SettingsFile { get; private set; }
        public string ExeName { get; private set; }
        public Dictionary<string, string> LoginFiles { get; private set; } = new();
        public List<string> ShortcutFolders { get; private set; } = new();
        public List<string> ShortcutIgnore { get; private set; } = new();
        public List<string> PathListToClear { get; private set; }
        public bool HasRegistryFiles { get; private set; }
        public void CurrentPlatformInit(string platform)
        {
            if (!(BasicPlatforms.Instance.PlatformExists(platform) || BasicPlatforms.Instance.PlatformExistsFromShort(platform))) return;

            _instance.FullName = platform;
            _instance.SafeName = Globals.GetCleanFilePath(platform);
            _instance.SettingsFile = _instance.SafeName + ".json";

            _instance.DefaultExePath = BasicSwitcherFuncs.ExpandEnvironmentVariables((string)BasicPlatforms.Instance.GetPlatformJson(platform)["ExeLocationDefault"]);
            _instance.ExeExtraArgs = (string)BasicPlatforms.Instance.GetPlatformJson(platform)["ExeExtraArgs"];
            _instance.DefaultFolderPath = BasicSwitcherFuncs.ExpandEnvironmentVariables(Path.GetDirectoryName(_instance.DefaultExePath));
            _instance.ExeName = Path.GetFileName(_instance.DefaultExePath);

            var jPlatform = BasicPlatforms.Instance.GetPlatformJson(platform);
            _instance._platformIds = jPlatform["Identifiers"]!.Values<string>().ToList();
            _instance.ExesToEnd = jPlatform["ExesToEnd"]!.Values<string>().ToList();

            _instance.LoginFiles.Clear();
            foreach (var (k,v) in jPlatform["LoginFiles"].ToObject<Dictionary<string, string>>())
            {
                _instance.LoginFiles.Add(BasicSwitcherFuncs.ExpandEnvironmentVariables(k), v);
                if (k.Contains("REG:")) _instance.HasRegistryFiles = true;
            }

            if (jPlatform.ContainsKey("PathListToClear"))
            {
                _instance.PathListToClear = jPlatform["PathListToClear"]!.Values<string>().ToList();
                if (_instance.PathListToClear.Contains("SAME_AS_LOGIN_FILES"))
                    _instance.PathListToClear = _instance.LoginFiles.Keys.ToList();
            }

            // Variables that may not be set:
            if (jPlatform.ContainsKey("UniqueIdFile")) _instance.UniqueIdFile = (string)jPlatform["UniqueIdFile"];
            if (jPlatform.ContainsKey("UniqueIdFolder")) _instance.UniqueIdFolder = (string)jPlatform["UniqueIdFolder"];
            if (jPlatform.ContainsKey("UniqueIdRegex")) _instance.UniqueIdRegex = Globals.ExpandRegex((string)jPlatform["UniqueIdRegex"]);
            if (jPlatform.ContainsKey("UniqueIdMethod")) _instance.UniqueIdMethod = Globals.ExpandRegex((string)jPlatform["UniqueIdMethod"]);
            if (jPlatform.ContainsKey("ExitBeforeInteract")) _instance.ExitBeforeInteract = (bool)jPlatform["ExitBeforeInteract"];
            if (jPlatform.ContainsKey("PeacefulExit")) _instance.PeacefulExit = (bool)jPlatform["PeacefulExit"];
            if (jPlatform.ContainsKey("ClearLoginCache")) _instance.ClearLoginCache = (bool)jPlatform["ClearLoginCache"];

            // Process "Extras"
            JObject extras = null;
            if (jPlatform.ContainsKey("Extras")) extras = (JObject)jPlatform["Extras"];
            if (extras != null)
            {
                _instance.HasExtras = true;
                if (extras.ContainsKey("UsernameModalExtraButtons"))
                    _instance.UsernameModalExtraButtons = (string)extras["UsernameModalExtraButtons"];
                if (extras.ContainsKey("UsernameModalCopyText"))
                    _instance.UserModalCopyText = (string)extras["UsernameModalCopyText"];
                if (extras.ContainsKey("UsernameModalCopyText"))
                    _instance.UserModalHintText = (string) extras["UsernameModalHintText"];
                if (extras.ContainsKey("CachePaths"))
                    _instance.CachePaths = extras["CachePaths"]!.Values<string>().ToList();
                if (extras.ContainsKey("ShortcutFolders"))
                    _instance.ShortcutFolders = extras["ShortcutFolders"]!.Values<string>().ToList();
                if (extras.ContainsKey("ShortcutIgnore"))
                    _instance.ShortcutIgnore = extras["ShortcutIgnore"]!.Values<string>().ToList();
            }

            // Foreach app shortcut in ShortcutFolders:
            // - Add as {<Name>: true} if not included in Basic.Instance.Shortcuts, and copy to cache shortcuts folder (each launch)
            // - If exists as true copy to cache shortcuts folder (each launch)
            // - If set to false: Ignore.
            //
            // These will be displayed next to the add new button in the switcher.
            // Maybe a pop-up menu if there are enough (Up arrow button on far right?)

            var sExisting = Basic.Instance.Shortcuts; // Existing shortcuts
            if (OperatingSystem.IsWindows())
                foreach (var sFolder in _instance.ShortcutFolders)
                {
                    // Foreach file in folder
                    foreach (var shortcut in new DirectoryInfo(BasicSwitcherFuncs.ExpandEnvironmentVariables(sFolder)).GetFiles())
                    {
                        var fName = shortcut.Name;
                        if (_instance.ShortcutIgnore.Contains(fName.Replace(".lnk", ""))) continue;

                        // Check if in saved shortcuts and If ignored
                        if (sExisting.ContainsKey(fName))
                        {
                            if (sExisting[fName] == -1) continue;
                        }
                        else
                            Basic.Instance.Shortcuts.Add(fName, 99); // Organization added later

                        var cachePath = Path.Join(PlatformLoginCache, "Shortcuts\\");
                        Directory.CreateDirectory(cachePath);
                        var outputShortcut = Path.Join(cachePath, fName);

                        // Extract image and place in wwwroot (Only if not already there):
                        if (!File.Exists(outputShortcut))
                        {
                            // Get full exe path from shortcut:


                            Globals.SaveIconFromFile(shortcut.FullName,
                                Path.Join(Globals.UserDataFolder,
                                    $"wwwroot\\img\\shortcuts\\{_instance.SafeName}\\{fName}"));
                        }

                        // Exists and is not ignored: Update shortcut
                        File.Copy(shortcut.FullName, outputShortcut, true);
                        // Organization will be saved in HTML/JS
                    }
                }

            // Either load existing, or safe default settings for platform
            if (File.Exists(Path.Join(Globals.UserDataFolder, _instance.SettingsFile))) Settings.Basic.Instance.LoadFromFile();
            else Settings.Basic.Instance.SaveSettings();

            _instance.IsInit = true;
        }
        // Variables that may not be set:
        public string LoginFileFromValue(string val) => BasicSwitcherFuncs.ExpandEnvironmentVariables(_instance.LoginFiles.FirstOrDefault(x => x.Value == val).Key);
        public string GetUniqueFilePath() => _instance.LoginFileFromValue(_instance.UniqueIdFile);
        #region OPTIONAL
        public string UniqueIdFile { get; private set; } = "";
        public string UniqueIdFolder { get; private set; } = "";
        public string UniqueIdRegex { get; private set; } = "";
        public string UniqueIdMethod { get; private set; } = "";
        public bool ExitBeforeInteract { get; private set; }
        public bool ClearLoginCache { get; private set; } = true;
        public bool PeacefulExit { get; private set; }
        public string PrimaryId => _platformIds[0];
        public string PlatformLoginCache => $"LoginCache\\{_instance.SafeName}\\";
        public string IdsJsonPath => Path.Join(_instance.PlatformLoginCache, "ids.json");
        public string AccountLoginCachePath(string acc) => Path.Join(_instance.PlatformLoginCache, "{acc}\\");
        #endregion

        #region EXTRAS
        public bool HasExtras { get; private set; } = false;
        public string UsernameModalExtraButtons { get; private set; } = "";
        public string UserModalCopyText { get; private set; } = "";
        public string UserModalHintText { get; private set; } = "";
        public List<string> CachePaths { get; private set; } = null;
        #endregion


        public Dictionary<string, string> ReadRegJson(string acc) =>
            GeneralFuncs.ReadDict(Path.Join(AccountLoginCachePath(acc), "reg.json"), true);
        public void SaveRegJson(Dictionary<string, string> regJson, string acc) =>
            GeneralFuncs.SaveDict(regJson, Path.Join(AccountLoginCachePath(acc), "reg.json"), true);

        public string GetUserModalExtraButtons => _instance.UsernameModalExtraButtons == "" ? "" :
            Globals.ReadAllText(Path.Join(Globals.AppDataFolder, _instance.UsernameModalExtraButtons));
        public string GetUserModalCopyText => _instance.UserModalCopyText == "" ? "" :
            Globals.ReadAllText(Path.Join(Globals.AppDataFolder, _instance.UserModalCopyText));

        public string GetUserModalHintText()
        {
            try
            {
                return Lang.Instance[_instance.UserModalHintText];
            }
            catch (Exception e)
            {
                return _instance.UserModalHintText;
            }
        }
    }
}
