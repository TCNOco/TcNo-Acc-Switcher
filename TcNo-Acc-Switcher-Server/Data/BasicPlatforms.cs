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
        public JToken GetPlatforms => (JObject)Instance._jData["Platforms"];
        private Dictionary<string, string> _platformDict = new();

        private readonly Dictionary<string, string> _platformDictAllPossible = new()
        {
            { "BattleNet", "Battle.Net" }
        };

        public BasicPlatforms() {}
        public void BasicPlatformsInit()
        {
            Instance._jData = GeneralFuncs.LoadSettings(Path.Join(Globals.AppDataFolder, "BasicPlatforms.json"));
            // Populate platform Primary Token to Full Name dictionary
            foreach (var jToken in GetPlatforms)
            {
                var x = (JProperty)jToken;
                var identifiers = GetPlatforms[x.Name]["Identifiers"].ToObject<List<string>>();
                foreach (var platformShort in identifiers)
                {
                    Instance._platformDictAllPossible.Add(platformShort, x.Name);
                }
                Instance._platformDict.Add(identifiers[0], x.Name);
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
            CurrentPlatform.Instance.CurrentPlatformInit(Instance.PlatformFullName(id));
        }
        public Dictionary<string, string> PlatformsDict => Instance._platformDict;

        private Dictionary<string, string> InactivePlatforms()
        {
            // Create local copy of platforms dict:
            var platforms = Instance._platformDict.ToDictionary(
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
        public string PrimaryIdFromPlatform(string platform) => Instance._platformDictAllPossible.FirstOrDefault(x => x.Value == platform).Key;
        public string GetExeNameFromPlatform(string platform) => Path.GetFileName((string)((JObject)GetPlatforms[platform])["ExeLocationDefault"]);
        public List<string> GetAllPrimaryIds() => Instance._platformDict.Keys.ToList();
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
        public bool ShortcutIncludeMainExe { get; private set; } = true;
        public bool SearchStartMenuForIcon { get; private set; } = false;
        public bool HasRegistryFiles { get; private set; }
        public void CurrentPlatformInit(string platform)
        {
            if (!(BasicPlatforms.Instance.PlatformExists(platform) || BasicPlatforms.Instance.PlatformExistsFromShort(platform))) return;

            Instance.FullName = platform;
            Instance.SafeName = Globals.GetCleanFilePath(platform);
            Instance.SettingsFile = Instance.SafeName + ".json";

            Instance.DefaultExePath = BasicSwitcherFuncs.ExpandEnvironmentVariables((string)BasicPlatforms.Instance.GetPlatformJson(platform)["ExeLocationDefault"]);
            Instance.ExeExtraArgs = (string)BasicPlatforms.Instance.GetPlatformJson(platform)["ExeExtraArgs"];
            Instance.DefaultFolderPath = BasicSwitcherFuncs.ExpandEnvironmentVariables(Path.GetDirectoryName(Instance.DefaultExePath));
            Instance.ExeName = Path.GetFileName(Instance.DefaultExePath);

            var jPlatform = BasicPlatforms.Instance.GetPlatformJson(platform);
            Instance._platformIds = jPlatform["Identifiers"]!.Values<string>().ToList();
            Instance.ExesToEnd = jPlatform["ExesToEnd"]!.Values<string>().ToList();

            Instance.LoginFiles.Clear();
            foreach (var (k,v) in jPlatform["LoginFiles"].ToObject<Dictionary<string, string>>())
            {
                Instance.LoginFiles.Add(BasicSwitcherFuncs.ExpandEnvironmentVariables(k), v);
                if (k.Contains("REG:")) Instance.HasRegistryFiles = true;
            }

            if (jPlatform.ContainsKey("PathListToClear"))
            {
                Instance.PathListToClear = jPlatform["PathListToClear"]!.Values<string>().ToList();
                if (Instance.PathListToClear.Contains("SAME_AS_LOGIN_FILES"))
                    Instance.PathListToClear = Instance.LoginFiles.Keys.ToList();
            }

            // Variables that may not be set:
            if (jPlatform.ContainsKey("UniqueIdFile")) Instance.UniqueIdFile = (string)jPlatform["UniqueIdFile"];
            if (jPlatform.ContainsKey("UniqueIdFolder")) Instance.UniqueIdFolder = (string)jPlatform["UniqueIdFolder"];
            if (jPlatform.ContainsKey("UniqueIdRegex")) Instance.UniqueIdRegex = Globals.ExpandRegex((string)jPlatform["UniqueIdRegex"]);
            if (jPlatform.ContainsKey("UniqueIdMethod")) Instance.UniqueIdMethod = Globals.ExpandRegex((string)jPlatform["UniqueIdMethod"]);
            if (jPlatform.ContainsKey("ExitBeforeInteract")) Instance.ExitBeforeInteract = (bool)jPlatform["ExitBeforeInteract"];
            if (jPlatform.ContainsKey("ClearLoginCache")) Instance.ClearLoginCache = (bool)jPlatform["ClearLoginCache"];

            // Process "Extras"
            JObject extras = null;
            if (jPlatform.ContainsKey("Extras")) extras = (JObject)jPlatform["Extras"];
            if (extras != null)
            {
                Instance.HasExtras = true;
                if (extras.ContainsKey("UsernameModalExtraButtons"))
                    Instance.UsernameModalExtraButtons = (string)extras["UsernameModalExtraButtons"];
                if (extras.ContainsKey("UsernameModalCopyText"))
                    Instance.UserModalCopyText = (string)extras["UsernameModalCopyText"];
                if (extras.ContainsKey("UsernameModalCopyText"))
                    Instance.UserModalHintText = (string) extras["UsernameModalHintText"];
                if (extras.ContainsKey("CachePaths"))
                    Instance.CachePaths = extras["CachePaths"]!.Values<string>().ToList();
                if (extras.ContainsKey("ShortcutFolders"))
                    Instance.ShortcutFolders = extras["ShortcutFolders"]!.Values<string>().ToList();
                if (extras.ContainsKey("ShortcutIgnore"))
                    Instance.ShortcutIgnore = extras["ShortcutIgnore"]!.Values<string>().ToList();
                if (extras.ContainsKey("ShortcutIncludeMainExe"))
                    Instance.ShortcutIncludeMainExe = (bool)extras["ShortcutIncludeMainExe"];
                if (extras.ContainsKey("SearchStartMenuForIcon"))
                    Instance.SearchStartMenuForIcon = (bool)extras["SearchStartMenuForIcon"];
            }

            // Foreach app shortcut in ShortcutFolders:
            // - Add as {<Name>: true} if not included in Basic.Instance.Shortcuts, and copy to cache shortcuts folder (each launch)
            // - If exists as true copy to cache shortcuts folder (each launch)
            // - If set to false: Ignore.
            //
            // These will be displayed next to the add new button in the switcher.
            // Maybe a pop-up menu if there are enough (Up arrow button on far right?)
            Basic.Instance.LoadFromFile(Instance.SettingsFile);

            var sExisting = Basic.Instance.Shortcuts; // Existing shortcuts
            if (Basic.Instance.Shortcuts == new Dictionary<int, string>()) // If nothing set, make sure it's loaded!
                Basic.Instance.LoadFromFile();

            if (OperatingSystem.IsWindows())
            {
                // Add image for main platform button:
                if (Instance.ShortcutIncludeMainExe)
                {
                    var imagePath = Path.Join(GetShortcutImagePath(), Instance.SafeName + ".png");

                    if (!File.Exists(imagePath))
                    {
                        if (Instance.SearchStartMenuForIcon)
                        {
                            var startMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuAppData%"), Instance.SafeName + ".lnk", SearchOption.AllDirectories);
                            var commonStartMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuProgramData%"), Instance.SafeName + ".lnk", SearchOption.AllDirectories);
                            if (startMenuFiles.Length > 0)
                                Globals.SaveIconFromFile(startMenuFiles[0], imagePath);
                            else if (commonStartMenuFiles.Length > 0)
                                Globals.SaveIconFromFile(commonStartMenuFiles[0], imagePath);
                            else
                                Globals.SaveIconFromFile(Basic.Instance.Exe(), imagePath);
                        }
                        else
                        {
                            Globals.SaveIconFromFile(Basic.Instance.Exe(), imagePath);
                        }
                    }
                }


                var cacheShortcuts = Instance.ShortcutFolder; // Shortcut cache
                foreach (var sFolder in Instance.ShortcutFolders)
                {
                    // Foreach file in folder
                    foreach (var shortcut in new DirectoryInfo(BasicSwitcherFuncs.ExpandEnvironmentVariables(sFolder)).GetFiles())
                    {
                        var fName = shortcut.Name;
                        if (Instance.ShortcutIgnore.Contains(RemoveShortcutExt(fName))) continue;

                        // Check if in saved shortcuts and If ignored
                        if (File.Exists(GetShortcutIgnoredPath(fName)))
                        {
                            var imagePath = Path.Join(GetShortcutImagePath(), RemoveShortcutExt(fName) + ".png");
                            if (File.Exists(imagePath)) File.Delete(imagePath);
                            fName = fName.Replace("_ignored", "");
                            if (Basic.Instance.Shortcuts.ContainsValue(fName))
                                Basic.Instance.Shortcuts.Remove(Basic.Instance.Shortcuts.First(e => e.Value == fName).Key);
                            continue;
                        }

                        Directory.CreateDirectory(cacheShortcuts);
                        var outputShortcut = Path.Join(cacheShortcuts, fName);

                        // Exists and is not ignored: Update shortcut
                        File.Copy(shortcut.FullName, outputShortcut, true);
                        // Organization will be saved in HTML/JS
                    }
                }

                // Now get images for all the shortcuts in the folder, as long as they don't already exist:
                List<string> existingShortcuts = new();
                if (Directory.Exists(cacheShortcuts))
                    foreach (var f in new DirectoryInfo(cacheShortcuts).GetFiles())
                    {
                        if (f.Name.Contains("_ignored")) continue;
                        var imageName = RemoveShortcutExt(f.Name) + ".png";
                        var imagePath = Path.Join(GetShortcutImagePath(), imageName);
                        existingShortcuts.Add(f.Name);
                        if (!Basic.Instance.Shortcuts.ContainsValue(f.Name))
                        {
                            // Not found in list, so add!
                            var last = Basic.Instance.Shortcuts.Count > 0 ? Basic.Instance.Shortcuts.Last().Key : -1;
                            last += 1;
                            Basic.Instance.Shortcuts.Add(last, f.Name); // Organization added later
                        }

                        // Extract image and place in wwwroot (Only if not already there):
                        if (!File.Exists(imagePath))
                        {
                            Globals.SaveIconFromFile(f.FullName, imagePath);
                        }
                    }

                foreach (var (i, s) in Basic.Instance.Shortcuts)
                {
                    if (!existingShortcuts.Contains(s))
                        Basic.Instance.Shortcuts.Remove(i);
                }
            }

            Basic.Instance.SaveSettings();

            Instance.IsInit = true;
        }
        // Variables that may not be set:
        public string LoginFileFromValue(string val) => BasicSwitcherFuncs.ExpandEnvironmentVariables(Instance.LoginFiles.FirstOrDefault(x => x.Value == val).Key);
        public string GetUniqueFilePath() => Instance.LoginFileFromValue(Instance.UniqueIdFile);
        #region OPTIONAL
        public string UniqueIdFile { get; private set; } = "";
        public string UniqueIdFolder { get; private set; } = "";
        public string UniqueIdRegex { get; private set; } = "";
        public string UniqueIdMethod { get; private set; } = "";
        public bool ExitBeforeInteract { get; private set; }
        public bool ClearLoginCache { get; private set; } = true;
        public string PrimaryId => _platformIds[0];
        public string ShortcutFolder => Path.Join(PlatformLoginCache, "Shortcuts\\");
        public string PlatformLoginCache => $"LoginCache\\{Instance.SafeName}\\";
        public string IdsJsonPath => Path.Join(Instance.PlatformLoginCache, "ids.json");
        public string AccountLoginCachePath(string acc) => Path.Join(Instance.PlatformLoginCache, $"{acc}\\");
        #endregion

        #region EXTRAS
        public bool HasExtras { get; private set; } = false;
        public string UsernameModalExtraButtons { get; private set; } = "";
        public string UserModalCopyText { get; private set; } = "";
        public string UserModalHintText { get; private set; } = "";
        public List<string> CachePaths { get; private set; } = null;
        #endregion

        public static string GetShortcutImageFolder => $"img\\shortcuts\\{Instance.SafeName}\\";
        public static string GetShortcutImagePath() => Path.Join(Globals.UserDataFolder, "wwwroot\\", GetShortcutImageFolder);
        public static string GetShortcutIgnoredPath(string shortcut) => Path.Join(Instance.ShortcutFolder, shortcut.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));

        public static string RemoveShortcutExt(string s) => s.Replace(".lnk", "").Replace(".url", "");
        public static string GetShortcutImagePath(string gameShortcutName) =>
            Path.Join(GetShortcutImageFolder, RemoveShortcutExt(gameShortcutName) + ".png");

        public Dictionary<string, string> ReadRegJson(string acc) =>
            GeneralFuncs.ReadDict(Path.Join(AccountLoginCachePath(acc), "reg.json"), true);
        public void SaveRegJson(Dictionary<string, string> regJson, string acc) =>
            GeneralFuncs.SaveDict(regJson, Path.Join(AccountLoginCachePath(acc), "reg.json"), true);

        public string GetUserModalExtraButtons => Instance.UsernameModalExtraButtons == "" ? "" :
            Globals.ReadAllText(Path.Join(Globals.AppDataFolder, Instance.UsernameModalExtraButtons));
        public string GetUserModalCopyText => Instance.UserModalCopyText == "" ? "" :
            Globals.ReadAllText(Path.Join(Globals.AppDataFolder, Instance.UserModalCopyText));

        public string GetUserModalHintText()
        {
            try
            {
                return Lang.Instance[Instance.UserModalHintText];
            }
            catch (Exception e)
            {
                return Instance.UserModalHintText;
            }
        }
    }
}
