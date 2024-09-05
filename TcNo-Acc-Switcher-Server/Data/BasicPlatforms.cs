using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Text.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data
{
    public sealed class BasicPlatforms
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
                    BasicPlatformsInit();
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
        // ---------------

        private JObject _jData;
        private readonly SortedDictionary<string, string> _platformDict = new();
        private readonly Dictionary<string, string> _platformDictAllPossible = new();

        private static JObject JData { get => Instance._jData; set => Instance._jData = value; }
        public static JToken GetPlatforms => (JObject)Instance._jData["Platforms"];
        public static SortedDictionary<string, string> PlatformDict => Instance._platformDict;
        public static Dictionary<string, string> PlatformDictAllPossible => Instance._platformDictAllPossible;
        public static JObject GetPlatformJson(string platform) => (JObject)GetPlatforms![platform];
        public static bool PlatformExists(string platform) => ((JObject)GetPlatforms).ContainsKey(platform);
        public static bool PlatformExistsFromShort(string id) => ((JObject)GetPlatforms).ContainsKey(PlatformFullName(id));

        /* ---------------
        public static void SetCurrentPlatform(string platform)
        {
            CurrentPlatform.Instance = new CurrentPlatform();
            CurrentPlatform.Instance.CurrentPlatformInit(platform);
        }
        */
        public static void SetCurrentPlatformFromShort(string id)
        {
            CurrentPlatform.Instance = new CurrentPlatform();
            CurrentPlatform.Instance.CurrentPlatformInit(PlatformFullName(id));
        }

        public static Dictionary<string, string> InactivePlatforms()
        {
            // Create local copy of platforms dict:
            var platforms = PlatformDict.ToDictionary(
                entry => entry.Key,
                entry => entry.Value);

            foreach (var enabledPlat in AppSettings.EnabledBasicPlatforms)
                platforms.Remove(enabledPlat);

            return platforms;
        }

        /// <summary>
        /// This is a LEGACY file, or comes with the installer. To be moved to the User data folder (See PlatformsPathUser)
        /// </summary>
        private static string PlatformsPathApp => Path.Join(Globals.AppDataFolder, "Platforms.json");

        /// <summary>
        /// This is the Platforms.json file
        /// </summary>
        private static string PlatformsPathUser => Path.Join(Globals.UserDataFolder, "Platforms.json");

        private static int GetPlatformsFileVersion(string platformsPath)
        {
            if (!File.Exists(platformsPath)) return 0;
            using JsonDocument document = JsonDocument.Parse(File.ReadAllText(platformsPath));
            JsonElement root = document.RootElement;
            return Globals.DateStringToInt(root.GetProperty("Version").GetString());
        }

        private static void BasicPlatformsInit()
        {
            // Check if Platforms.json exists in User folder or App folder, and if both: Get the latest version.
            // Should only be one copy: In the User Data folder.
            var userPlatformsVersion = GetPlatformsFileVersion(PlatformsPathUser);
            var appPlatformsVersion = GetPlatformsFileVersion(PlatformsPathApp);
            if (appPlatformsVersion != 0 || userPlatformsVersion != 0)
            {
                // Either file exists, so compare and move. Should only be one.
                if (appPlatformsVersion > userPlatformsVersion)
                {
                    // Move more up-to-date Platforms.json from app to user folder
                    if (Globals.CopyFile(PlatformsPathApp, PlatformsPathUser)) Globals.DeleteFile(PlatformsPathApp);
                }

                // Delete App Platforms.json
                if (File.Exists(PlatformsPathApp)) Globals.DeleteFile(PlatformsPathApp);
            }

            // Update, if possible.
            UpdatePlatformsJson();

            // First attempt: Load
            if (!TryLoadPlatforms())
            {
                // Delete
                if (File.Exists(PlatformsPathUser)) Globals.DeleteFile(PlatformsPathUser);

                // Redownload
                UpdatePlatformsJson();

                if (!TryLoadPlatforms())
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_FailedPlatformsLoad"],
                        renderTo: "toastarea");
                }
            }
        }

        private static bool TryLoadPlatforms()
        {
            try
            {
                JData = GeneralFuncs.LoadSettings(PlatformsPathUser);
                // Populate platform Primary Token to Full Name dictionary
                foreach (var jToken in GetPlatforms)
                {
                    var x = (JProperty)jToken;
                    var identifiers = GetPlatforms[x.Name]?["Identifiers"]?.ToObject<List<string>>();
                    if (identifiers == null) continue;
                    foreach (var platformShort in identifiers)
                    {
                        PlatformDictAllPossible.Add(platformShort, x.Name);
                    }

                    PlatformDict.Add(identifiers[0], x.Name);
                }

                return true;
            } catch (Exception e) {
                Globals.WriteToLog("Failed to locate Platforms.json! This will cause a lot of features to break. If this is the first attempt: Another copy will be downloaded.", e);
            }

            return false;
        }

        public async static void UpdatePlatformsJson()
        {
            try
            {
                if (!AppSettings.AutoUpdatePlatforms)
                {
                    Globals.WriteToLog("Can not check for updated Platforms.json, as OfflineMode is enabled.");
                    return;
                }

                // Get current version
                var currentPlatformsVersion = 0;
                var latestPlatformsVersion = 0;
                if (File.Exists(PlatformsPathUser))
                {
                    using JsonDocument document = JsonDocument.Parse(File.ReadAllText(PlatformsPathUser));
                    JsonElement root = document.RootElement;

                    if (root.TryGetProperty("Version", out JsonElement versionElement))
                        currentPlatformsVersion = Globals.DateStringToInt(versionElement.GetString());
                }

                // Get latest version from GitHub
                using HttpClient client = new();
                string currentDateTimeStamp = DateTime.Now.ToString("yyyyMMddHHmmss");
                string latestPlatformsString = await client.GetStringAsync("https://raw.githubusercontent.com/TCNOco/TcNo-Acc-Switcher/master/TcNo-Acc-Switcher-Server/Platforms.json?{currentDateTimeStamp}");

                using (JsonDocument document = JsonDocument.Parse(latestPlatformsString))
                {
                    JsonElement root = document.RootElement;

                    if (root.TryGetProperty("Version", out JsonElement versionElement))
                        latestPlatformsVersion = Globals.DateStringToInt(versionElement.GetString());
                }

                // Compare and copy if newer
                if (latestPlatformsVersion > currentPlatformsVersion)
                {
                    Globals.DeleteFile(PlatformsPathUser + ".bak");
                    Globals.CopyFile(PlatformsPathUser, PlatformsPathUser + ".bak");
                    Globals.WriteToLog("Updated Platforms.json");

                    using StreamWriter sw = new(PlatformsPathUser);
                    await sw.WriteAsync(latestPlatformsString);
                }

                // Download missing images
                var newPlatforms = "";
                using (JsonDocument document = JsonDocument.Parse(File.ReadAllText(PlatformsPathUser)))
                {
                    JsonElement root = document.RootElement;

                    // Iterate over root > "Platforms". Each key is the name we're looking for.
                    foreach (var platform in root.GetProperty("Platforms").EnumerateObject())
                    {
                        string platformName = platform.Name;
                        string platformImage = Globals.GetCleanFilePath(platformName);
                        string platformImageUrl = $"https://raw.githubusercontent.com/TCNOco/TcNo-Acc-Switcher/master/TcNo-Acc-Switcher-Server/wwwroot/img/platform/{platformImage}.svg";

                        if (string.IsNullOrEmpty(platformImage)) continue;

                        string imagePath = Path.Join(Globals.UserDataFolder, "wwwroot", "img", "platform", platformImage + ".svg");

                        if (!File.Exists(imagePath))
                        {
                            try
                            {
                                // Download latest image from GitHub:
                                byte[] imageBytes = await client.GetByteArrayAsync(platformImageUrl);

                                using FileStream fs = new(imagePath, FileMode.Create);
                                await fs.WriteAsync(imageBytes);

                                newPlatforms += platformName + ", ";
                            }
                            catch (Exception ex)
                            {
                                Globals.WriteToLog($"Failed to download image for {platformName}: {ex.Message}");
                            }
                        }
                    }
                }

                if (newPlatforms.Length != 0)
                {
                    _ = GeneralInvocableFuncs.ShowToast("info", Lang.Instance["Toast_NewPlatforms", new { listOfNames = newPlatforms }], renderTo: "toastarea", duration: 10000);

                    Globals.WriteToLog($"Updated Platforms.json. Added new platforms: {newPlatforms}");
                }
            }
            catch (Exception ex)
            {
                Globals.WriteToLog($"Failed to update Platforms.json: {ex.Message}");
            }
        }

        public static string PlatformFullName(string id) => PlatformDict.ContainsKey(id) ? PlatformDict[id] : id;
        public static string PlatformSafeName(string id) =>
            PlatformDict.ContainsKey(id) ? Globals.GetCleanFilePath(PlatformDict[id]) : id;
        public static string PrimaryIdFromPlatform(string platform) => PlatformDictAllPossible.FirstOrDefault(x => x.Value == platform).Key;
        public static string GetExeNameFromPlatform(string platform) => Path.GetFileName((string)((JObject)GetPlatforms[platform])?["ExeLocationDefault"]);
    }

    public sealed class CurrentPlatform
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
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }
        // ---------------
        #region Backing Fields
        private List<string> _platformIds;
        private List<string> _exesToEnd;
        private bool _isInit;
        private string _fullName;
        private string _defaultExePath;
        private string _exeExtraArgs = "";
        private string _defaultFolderPath;
        private string _getPathFromShortcutNamed = "";
        private string _safeName;
        private string _settingsFile;
        private string _exeName;
        private Dictionary<string, string> _loginFiles = new();
        private bool _allFilesRequired = false;
        private List<string> _shortcutFolders = new();
        private List<string> _shortcutIgnore = new();
        private List<string> _pathListToClear;
        private bool _shortcutIncludeMainExe = true;
        private bool _searchStartMenuForIcon;
        private bool _hasRegistryFiles;
        // Extras
        private bool _hasExtras;
        private string _usernameModalExtraButtons = "";
        private string _userModalCopyText = "";
        private string _userModalHintText = "";
        private List<string> _cachePaths;
        private string _profilePicFromFile = "";
        private string _profilePicPath = "";
        private string _profilePicRegex = "";
        private Dictionary<string, string> _backupPaths = new();
        private List<string> _backupFileTypesIgnore = new();
        private List<string> _backupFileTypesInclude = new();
        private string _closingMethod = "Combined";
        private string _startingMethod = "Default";
        private bool _regDeleteOnClear;

        // Optional
        private string _uniqueIdFile = "";
        private string _uniqueIdFolder = "";
        private string _uniqueIdRegex = "";
        private string _uniqueIdMethod = "";
        private bool _exitBeforeInteract;
        private bool _clearLoginCache = true;
        #endregion
        #region Properties
        public static List<string> PlatformIds { get => Instance._platformIds; set => Instance._platformIds = value; }
        public static List<string> ExesToEnd { get => Instance._exesToEnd; private set => Instance._exesToEnd = value; }
        public static bool IsInit { get => Instance._isInit; private set => Instance._isInit = value; }
        public static string FullName { get => Instance._fullName; private set => Instance._fullName = value; }
        public static string DefaultExePath { get => Instance._defaultExePath; private set => Instance._defaultExePath = value; }
        public static string ExeExtraArgs { get => Instance._exeExtraArgs; private set => Instance._exeExtraArgs = value; }
        public static string DefaultFolderPath { get => Instance._defaultFolderPath; private set => Instance._defaultFolderPath = value; }
        public static string GetPathFromShortcutNamed { get => Instance._getPathFromShortcutNamed; private set => Instance._getPathFromShortcutNamed = value; }
        public static string SafeName { get => Instance._safeName; private set => Instance._safeName = value; }
        public static string SettingsFile { get => Instance._settingsFile; private set => Instance._settingsFile = value; }
        public static string ExeName { get => Instance._exeName; private set => Instance._exeName = value; }
        public static Dictionary<string, string> LoginFiles => Instance._loginFiles;
        public static bool AllFilesRequired { get => Instance._allFilesRequired; private set => Instance._allFilesRequired = value; }
        public static List<string> ShortcutFolders { get => Instance._shortcutFolders; private set => Instance._shortcutFolders = value; }
        public static List<string> ShortcutIgnore { get => Instance._shortcutIgnore; private set => Instance._shortcutIgnore = value; }
        public static List<string> PathListToClear { get => Instance._pathListToClear; private set => Instance._pathListToClear = value; }
        public static bool ShortcutIncludeMainExe { get => Instance._shortcutIncludeMainExe; private set => Instance._shortcutIncludeMainExe = value; }
        public static bool SearchStartMenuForIcon { get => Instance._searchStartMenuForIcon; private set => Instance._searchStartMenuForIcon = value; }
        public static bool HasRegistryFiles { get => Instance._hasRegistryFiles; private set => Instance._hasRegistryFiles = value; }

        // Optional
        public static string UniqueIdFile { get => Instance._uniqueIdFile; private set => Instance._uniqueIdFile = value; }
        public static string UniqueIdFolder { get => Instance._uniqueIdFolder; private set => Instance._uniqueIdFolder = value; }
        public static string UniqueIdRegex { get => Instance._uniqueIdRegex; private set => Instance._uniqueIdRegex = value; }
        public static string UniqueIdMethod { get => Instance._uniqueIdMethod; private set => Instance._uniqueIdMethod = value; }
        public static bool ExitBeforeInteract { get => Instance._exitBeforeInteract; private set => Instance._exitBeforeInteract = value; }
        public static bool ClearLoginCache { get => Instance._clearLoginCache; private set => Instance._clearLoginCache = value; }

        // Extras
        public static bool HasExtras { get => Instance._hasExtras; private set => Instance._hasExtras = value; }
        public static string UsernameModalExtraButtons { get => Instance._usernameModalExtraButtons; private set => Instance._usernameModalExtraButtons = value; }
        public static string UserModalCopyText { get => Instance._userModalCopyText; private set => Instance._userModalCopyText = value; }
        public static string UserModalHintText { get => Instance._userModalHintText; private set => Instance._userModalHintText = value; }
        public static List<string> CachePaths { get => Instance._cachePaths; private set => Instance._cachePaths = value; }
        public static string ProfilePicFromFile { get => Instance._profilePicFromFile; private set => Instance._profilePicFromFile = value; }
        public static string ProfilePicPath { get => Instance._profilePicPath; private set => Instance._profilePicPath = value; }
        public static string ProfilePicRegex { get => Instance._profilePicRegex; private set => Instance._profilePicRegex = value; }
        public static Dictionary<string, string> BackupPaths => Instance._backupPaths;
        public static List<string> BackupFileTypesIgnore { get => Instance._backupFileTypesIgnore; private set => Instance._backupFileTypesIgnore = value; }
        public static List<string> BackupFileTypesInclude { get => Instance._backupFileTypesInclude; private set => Instance._backupFileTypesInclude = value; }
        public static string ClosingMethod { get => Instance._closingMethod; set => Instance._closingMethod = value; }
        public static string StartingMethod { get => Instance._startingMethod; set => Instance._startingMethod = value; }
        public static bool RegDeleteOnClear { get => Instance._regDeleteOnClear; set => Instance._regDeleteOnClear = value; }

        // ----------
        #endregion

        public void CurrentPlatformInit(string platform)
        {
            if (!(BasicPlatforms.PlatformExists(platform) || BasicPlatforms.PlatformExistsFromShort(platform))) return;

            FullName = platform;
            SafeName = Globals.GetCleanFilePath(platform);
            SettingsFile = Path.Join("Settings\\", SafeName + ".json");

            DefaultExePath = BasicSwitcherFuncs.ExpandEnvironmentVariables((string)BasicPlatforms.GetPlatformJson(platform)["ExeLocationDefault"]);
            ExeExtraArgs = (string)BasicPlatforms.GetPlatformJson(platform)["ExeExtraArgs"] ?? "";
            DefaultFolderPath = BasicSwitcherFuncs.ExpandEnvironmentVariables(Path.GetDirectoryName(DefaultExePath));
            ExeName = Path.GetFileName(DefaultExePath);

            var jPlatform = BasicPlatforms.GetPlatformJson(platform);
            PlatformIds = jPlatform["Identifiers"]!.Values<string>().ToList();
            ExesToEnd = jPlatform["ExesToEnd"]!.Values<string>().ToList();

            LoginFiles.Clear();
            foreach (var (k,v) in jPlatform["LoginFiles"]?.ToObject<Dictionary<string, string>>()!)
            {
                LoginFiles.Add(BasicSwitcherFuncs.ExpandEnvironmentVariables(k), v);
                if (k.Contains("REG:")) HasRegistryFiles = true;
            }

            if (jPlatform.ContainsKey("PathListToClear"))
            {
                PathListToClear = jPlatform["PathListToClear"]!.Values<string>().ToList();
                if (PathListToClear.Contains("SAME_AS_LOGIN_FILES"))
                    PathListToClear = LoginFiles.Keys.ToList();
            }

            // Variables that may not be set:
            if (jPlatform.ContainsKey("GetPathFromShortcutNamed")) GetPathFromShortcutNamed = (string)jPlatform["GetPathFromShortcutNamed"];
            if (jPlatform.ContainsKey("UniqueIdFile")) UniqueIdFile = (string)jPlatform["UniqueIdFile"];
            if (jPlatform.ContainsKey("UniqueIdFolder")) UniqueIdFolder = (string)jPlatform["UniqueIdFolder"];
            if (jPlatform.ContainsKey("UniqueIdRegex")) UniqueIdRegex = Globals.ExpandRegex((string)jPlatform["UniqueIdRegex"]);
            if (jPlatform.ContainsKey("UniqueIdMethod")) UniqueIdMethod = Globals.ExpandRegex((string)jPlatform["UniqueIdMethod"]);
            if (jPlatform.ContainsKey("ExitBeforeInteract")) ExitBeforeInteract = (bool)jPlatform["ExitBeforeInteract"];
            if (jPlatform.ContainsKey("ClearLoginCache")) ClearLoginCache = (bool)jPlatform["ClearLoginCache"];
            if (jPlatform.ContainsKey("RegDeleteOnClear")) RegDeleteOnClear = (bool)jPlatform["RegDeleteOnClear"];

            // Process "Extras"
            JObject extras = null;
            if (jPlatform.ContainsKey("Extras")) extras = (JObject)jPlatform["Extras"];
            if (extras != null)
            {
                HasExtras = true;
                if (extras.ContainsKey("UsernameModalExtraButtons"))
                    UsernameModalExtraButtons = (string)extras["UsernameModalExtraButtons"];
                if (extras.ContainsKey("UsernameModalCopyText"))
                    UserModalCopyText = (string)extras["UsernameModalCopyText"];
                if (extras.ContainsKey("UsernameModalCopyText"))
                    UserModalHintText = (string) extras["UsernameModalHintText"];

                // Extras - Cache clearing
                if (extras.ContainsKey("CachePaths"))
                    CachePaths = extras["CachePaths"]!.Values<string>().ToList();

                // Extras - Shortcuts
                if (extras.ContainsKey("ShortcutFolders"))
                    ShortcutFolders = extras["ShortcutFolders"]!.Values<string>().ToList();
                if (extras.ContainsKey("ShortcutIgnore"))
                    ShortcutIgnore = extras["ShortcutIgnore"]!.Values<string>().ToList();
                if (extras.ContainsKey("ShortcutIncludeMainExe"))
                    ShortcutIncludeMainExe = (bool)extras["ShortcutIncludeMainExe"];
                if (extras.ContainsKey("SearchStartMenuForIcon"))
                    SearchStartMenuForIcon = (bool)extras["SearchStartMenuForIcon"];

                // Extras - Profile pic
                if (extras.ContainsKey("ProfilePicFromFile"))
                    ProfilePicFromFile = (string)extras["ProfilePicFromFile"];
                if (extras.ContainsKey("ProfilePicPath"))
                    ProfilePicPath = (string)extras["ProfilePicPath"];
                if (extras.ContainsKey("ProfilePicRegex"))
                    ProfilePicRegex = (string)extras["ProfilePicRegex"];

                // Extras - Backups
                if (extras.ContainsKey("BackupFolders"))
                {
                    BackupPaths.Clear();
                    foreach (var (k, v) in extras["BackupFolders"].ToObject<Dictionary<string, string>>())
                    {
                        BackupPaths.Add(BasicSwitcherFuncs.ExpandEnvironmentVariables(k), v);
                    }
                }
                if (extras.ContainsKey("BackupFileTypesIgnore"))
                    BackupFileTypesIgnore = extras["BackupFileTypesIgnore"]!.Values<string>().ToList();
                if (extras.ContainsKey("BackupFileTypesInclude"))
                    BackupFileTypesInclude = extras["BackupFileTypesInclude"]!.Values<string>().ToList();
                if (extras.ContainsKey("ClosingMethod"))
                    ClosingMethod = (string)extras["ClosingMethod"];
            }

            // Foreach app shortcut in ShortcutFolders:
            // - Add as {<Name>: true} if not included in Basic.Shortcuts, and copy to cache shortcuts folder (each launch)
            // - If exists as true copy to cache shortcuts folder (each launch)
            // - If set to false: Ignore.
            //
            // These will be displayed next to the add new button in the switcher.
            // Maybe a pop-up menu if there are enough (Up arrow button on far right?)

            if (OperatingSystem.IsWindows())
            {
                // Add image for main platform button:
                if (ShortcutIncludeMainExe)
                {
                    var imagePath = Path.Join(GetShortcutImagePath(), SafeName + ".png");

                    if (!File.Exists(imagePath))
                    {
                        if (SearchStartMenuForIcon)
                        {
                            var startMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuAppData%"), SafeName + ".lnk", SearchOption.AllDirectories);
                            var commonStartMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuProgramData%"), SafeName + ".lnk", SearchOption.AllDirectories);
                            if (startMenuFiles.Length > 0)
                                Globals.SaveIconFromFile(startMenuFiles[0], imagePath);
                            else if (commonStartMenuFiles.Length > 0)
                                Globals.SaveIconFromFile(commonStartMenuFiles[0], imagePath);
                            else
                                Globals.SaveIconFromFile(Basic.Exe(), imagePath);
                        }
                        else
                        {
                            Globals.SaveIconFromFile(Basic.Exe(), imagePath);
                        }
                    }
                }


                var cacheShortcuts = ShortcutFolder; // Shortcut cache
                foreach (var sFolder in ShortcutFolders)
                {
                    if (sFolder == "") continue;
                    // Foreach file in folder
                    var desktopShortcutFolder = BasicSwitcherFuncs.ExpandEnvironmentVariables(sFolder, true);
                    if (!Directory.Exists(desktopShortcutFolder)) continue;
                    foreach (var shortcut in new DirectoryInfo(desktopShortcutFolder).GetFiles())
                    {
                        var fName = shortcut.Name;
                        if (ShortcutIgnore.Contains(PlatformFuncs.RemoveShortcutExt(fName))) continue;

                        // Check if in saved shortcuts and If ignored
                        if (File.Exists(GetShortcutIgnoredPath(fName)))
                        {
                            var imagePath = Path.Join(GetShortcutImagePath(), PlatformFuncs.RemoveShortcutExt(fName) + ".png");
                            if (File.Exists(imagePath)) File.Delete(imagePath);
                            fName = fName.Replace("_ignored", "");
                            if (Basic.Shortcuts.ContainsValue(fName))
                                Basic.Shortcuts.Remove(Basic.Shortcuts.First(e => e.Value == fName).Key);
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
                        var imageName = PlatformFuncs.RemoveShortcutExt(f.Name) + ".png";
                        var imagePath = Path.Join(GetShortcutImagePath(), imageName);
                        existingShortcuts.Add(f.Name);
                        if (!Basic.Shortcuts.ContainsValue(f.Name))
                        {
                            // Not found in list, so add!
                            var last = 0;
                            foreach (var (k,_) in Basic.Shortcuts)
                                if (k > last) last = k;
                            last += 1;
                            Basic.Shortcuts.Add(last, f.Name); // Organization added later
                        }

                        // Extract image and place in wwwroot (Only if not already there):
                        if (!File.Exists(imagePath))
                        {
                            Globals.SaveIconFromFile(f.FullName, imagePath);
                        }
                    }

                foreach (var (i, s) in Basic.Shortcuts)
                {
                    if (!existingShortcuts.Contains(s))
                        Basic.Shortcuts.Remove(i);
                }
            }

            AppStats.SetGameShortcutCount(SafeName, Basic.Shortcuts);
            Basic.SaveSettings();

            IsInit = true;
        }

        // Variables that may not be set:
        public static string GetUniqueFilePath() => BasicSwitcherFuncs.ExpandEnvironmentVariables(UniqueIdFile);
        public static string GetShortcutImageFolder => $"img\\shortcuts\\{SafeName}\\";
        public static string GetShortcutImagePath() => Path.Join(Globals.UserDataFolder, "wwwroot\\", GetShortcutImageFolder);
        public static string GetShortcutIgnoredPath(string shortcut) => Path.Join(ShortcutFolder, shortcut.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));
        public static string PrimaryId => PlatformIds[0];
        public static string ShortcutFolder => Path.Join(PlatformLoginCache, "Shortcuts\\");
        public static string PlatformLoginCache => $"LoginCache\\{SafeName}\\";
        public static string IdsJsonPath => Path.Join(PlatformLoginCache, "ids.json");
        public static string AccountLoginCachePath(string acc) => Path.Join(PlatformLoginCache, $"{acc}\\");
        public static string GetShortcutImagePath(string gameShortcutName) =>
            Path.Join(GetShortcutImageFolder, PlatformFuncs.RemoveShortcutExt(gameShortcutName) + ".png");

        public static Dictionary<string, string> ReadRegJson(string acc) =>
            GeneralFuncs.ReadDict(Path.Join(AccountLoginCachePath(acc), "reg.json"), true);

        public static void SaveRegJson(Dictionary<string, string> regJson, string acc)
        {
            if (regJson.Count > 0)
                GeneralFuncs.SaveDict(regJson, Path.Join(AccountLoginCachePath(acc), "reg.json"), true);
        }


        public static string GetUserModalExtraButtons => UsernameModalExtraButtons == "" ? "" :
            Globals.ReadAllText(Path.Join(Globals.AppDataFolder, UsernameModalExtraButtons));
        public static string GetUserModalCopyText => UserModalCopyText == "" ? "" :
            Globals.ReadAllText(Path.Join(Globals.AppDataFolder, UserModalCopyText));

        public static string GetUserModalHintText()
        {
            try
            {
                return Lang.Instance[UserModalHintText];
            }
            catch (Exception)
            {
                return UserModalHintText;
            }
        }
    }

    /// <summary>
    /// A place for all the basic functions, such as text replacement.
    /// </summary>
    public static class PlatformFuncs
    {
        public static string RemoveShortcutExt(string s) => !string.IsNullOrWhiteSpace(s) ? s.Replace(".lnk", "").Replace(".url", "") : "";
    }
}
