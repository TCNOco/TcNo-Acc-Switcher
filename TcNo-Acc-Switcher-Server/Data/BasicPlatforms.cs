using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data
{
    public interface IBasicPlatforms
    {
        JToken GetPlatforms { get; }
        JObject GetPlatformJson(string platform);
        void SetCurrentPlatform(string id);
    }

    public sealed class BasicPlatforms : IBasicPlatforms
    {
        [Inject] private IAppSettings AppSettings { get; }
        [Inject] private IAppData AppData { get; }
        [Inject] private ILang Lang { get; }
        [Inject] private ICurrentPlatform CurrentPlatform { get; }
        [Inject] private IGeneralFuncs GeneralFuncs { get; }

        public BasicPlatforms()
        {
            BasicPlatformsInit().GetAwaiter().GetResult();
        }

        private JObject JData { get; set; }
        public JToken GetPlatforms => (JObject)JData["Platforms"];
        public JObject GetPlatformJson(string platform) => (JObject)GetPlatforms![platform];

        public void SetCurrentPlatform(string id)
        {
            CurrentPlatform.Reset();
            var fullName = AppSettings.GetPlatform(id)?.Name;
            if (fullName == null) return;
            CurrentPlatform.CurrentPlatformInit(fullName);
            AppData.CurrentSwitcher = fullName;
        }

        private async Task BasicPlatformsInit()
        {
            // Check if Platforms.json exists.
            // If it doesnt: Copy it from the programs' folder to the user data folder.
            var basicPlatformsPath = Path.Join(Globals.AppDataFolder, "Platforms.json");
            if (!File.Exists(basicPlatformsPath))
            {
                // Once again verify the file exists. If it doesn't throw an error here.
                await GeneralFuncs.ShowToast("error", Lang["Toast_FailedPlatformsLoad"],
                    renderTo: "toastarea");
                Globals.WriteToLog("Failed to locate Platforms.json! This will cause a lot of features to break.");
                return;
            }

            JData = GeneralFuncs.LoadSettings(Path.Join(Globals.AppDataFolder, "Platforms.json"));
            // Populate platform Primary Token to Full Name dictionary
            foreach (var jToken in GetPlatforms)
            {
                var x = (JProperty)jToken;
                var identifiers = GetPlatforms[x.Name]?["Identifiers"]?.ToObject<List<string>>();
                if (identifiers == null) continue;
                var exeName = Path.GetFileName((string) ((JObject) GetPlatforms[x.Name])?["ExeLocationDefault"]);
                // Add to master platform list
                if (AppSettings.Platforms.Count(y => y.Name == x.Name) == 0)
                    AppSettings.Platforms.Add(new AppSettings.PlatformItem(x.Name, identifiers, exeName, false));
                else
                {
                    // Make sure that everything is set up properly.
                    var platform = AppSettings.Platforms.First(y => y.Name == x.Name);
                    platform.SetFromPlatformItem(new AppSettings.PlatformItem(x.Name, identifiers, exeName, false));
                }
            }
        }
    }

    public interface ICurrentPlatform
    {
        public void Reset();
        List<string> PlatformIds { get; set; }
        List<string> ExesToEnd { get; set; }
        bool IsInit { get; set; }
        string FullName { get; set; }
        string DefaultExePath { get; set; }
        string ExtraArgs { get; set; }
        string DefaultFolderPath { get; set; }
        string SafeName { get; set; }
        string SettingsFile { get; set; }
        string ExeName { get; set; }
        Dictionary<string, string> LoginFiles { get; init; }
        List<string> ShortcutFolders { get; set; }
        List<string> ShortcutIgnore { get; set; }
        List<string> PathListToClear { get; set; }
        bool ShortcutIncludeMainExe { get; set; }
        bool SearchStartMenuForIcon { get; set; }
        bool HasRegistryFiles { get; set; }
        string UniqueIdFile { get; set; }
        string UniqueIdFolder { get; set; }
        string UniqueIdRegex { get; set; }
        string UniqueIdMethod { get; set; }
        bool ExitBeforeInteract { get; set; }
        bool ClearLoginCache { get; set; }
        bool HasExtras { get; set; }
        string UsernameModalExtraButtons { get; set; }
        string UserModalCopyText { get; set; }
        string UserModalHintText { get; set; }
        List<string> CachePaths { get; set; }
        string ProfilePicFromFile { get; set; }
        string ProfilePicPath { get; set; }
        string ProfilePicRegex { get; set; }
        Dictionary<string, string> BackupPaths { get; init; }
        List<string> BackupFileTypesIgnore { get; set; }
        List<string> BackupFileTypesInclude { get; set; }
        string ClosingMethod { get; set; }
        string StartingMethod { get; set; }
        bool RegDeleteOnClear { get; set; }
        string GetShortcutImageFolder { get; }
        string PrimaryId { get; }
        string ShortcutFolder { get; }
        string PlatformLoginCache { get; }
        string IdsJsonPath { get; }
        MarkupString GetUserModalExtraButtons { get; }
        string GetUserModalCopyText { get; }
        void CurrentPlatformInit(string requestedPlatform);
        string GetUniqueFilePath();
        string GetShortcutImagePath();
        string GetShortcutImagePath(string gameShortcutName);
        string GetShortcutIgnoredPath(string shortcut);
        string AccountLoginCachePath(string acc);
        Dictionary<string, string> ReadRegJson(string acc);
        void SaveRegJson(Dictionary<string, string> regJson, string acc);
        string GetUserModalHintText();
    }

    public sealed class CurrentPlatform : ICurrentPlatform
    {
        [Inject] private IGeneralFuncs GeneralFuncs { get; }
        [Inject] private IAppSettings AppSettings { get; }
        [Inject] private IBasic Basic { get; }
        [Inject] private IBasicPlatforms BasicPlatforms { get; }
        [Inject] private ILang Lang { get; }
        [Inject] private IAppStats AppStats { get; }

        public CurrentPlatform(){}

        #region Properties
        public List<string> PlatformIds { get; set; }
        public List<string> ExesToEnd { get; set; }
        public bool IsInit { get; set; }
        public string FullName { get; set; }
        public string DefaultExePath { get; set; }
        public string ExtraArgs { get; set; } = "";
        public string DefaultFolderPath { get; set; }
        public string SafeName { get; set; }
        public string SettingsFile { get; set; }
        public string ExeName { get; set; }
        public Dictionary<string, string> LoginFiles { get; init; } = new();
        public List<string> ShortcutFolders { get; set; } = new();
        public List<string> ShortcutIgnore { get; set; } = new();
        public List<string> PathListToClear { get; set; }
        public bool ShortcutIncludeMainExe { get; set; } = true;
        public bool SearchStartMenuForIcon { get; set; }
        public bool HasRegistryFiles { get; set; }

        // Optional
        public string UniqueIdFile { get; set; } = "";
        public string UniqueIdFolder { get; set; } = "";
        public string UniqueIdRegex { get; set; } = "";
        public string UniqueIdMethod { get; set; } = "";
        public bool ExitBeforeInteract { get; set; }
        public bool ClearLoginCache { get; set; } = true;

        // Extras
        public bool HasExtras { get; set; }
        public string UsernameModalExtraButtons { get; set; } = "";
        public string UserModalCopyText { get; set; } = "";
        public string UserModalHintText { get; set; } = "";
        public List<string> CachePaths { get; set; }
        public string ProfilePicFromFile { get; set; } = "";
        public string ProfilePicPath { get; set; } = "";
        public string ProfilePicRegex { get; set; } = "";
        public Dictionary<string, string> BackupPaths { get; init; } = new();
        public List<string> BackupFileTypesIgnore { get; set; } = new();
        public List<string> BackupFileTypesInclude { get; set; } = new();
        public string ClosingMethod { get; set; } = "Combined";
        public string StartingMethod { get; set; } = "Default";
        public bool RegDeleteOnClear { get; set; }

        // ----------
        #endregion

        public void Reset()
        {
            var type = GetType();
            var properties = type.GetProperties();
            foreach (var t in properties)
                t.SetValue(this, null);
        }

        public void CurrentPlatformInit(string requestedPlatform)
        {
            var platform =
                AppSettings.Platforms.FirstOrDefault(x => x.Name == requestedPlatform || x.PossibleIdentifiers.Contains(requestedPlatform));
            if (platform is null) return;


            FullName = platform.Name;
            SafeName = platform.SafeName;
            SettingsFile = Path.Join("Settings\\", SafeName + ".json");

            var jPlatform = BasicPlatforms.GetPlatformJson(platform.Name);

            DefaultExePath = Basic.ExpandEnvironmentVariables((string)jPlatform["ExeLocationDefault"]);
            ExeModalData.ExtraArgs = (string)jPlatform["ExeModalData.ExtraArgs"] ?? "";
            DefaultFolderPath = Basic.ExpandEnvironmentVariables(Path.GetDirectoryName(DefaultExePath));
            ExeName = platform.ExeName;

            PlatformIds = jPlatform["Identifiers"]!.Values<string>().ToList();
            ExesToEnd = jPlatform["ExesToEnd"]!.Values<string>().ToList();

            LoginFiles.Clear();
            foreach ((string k, string v) in jPlatform["LoginFiles"]?.ToObject<Dictionary<string, string>>()!)
            {
                LoginFiles.Add(Basic.ExpandEnvironmentVariables(k), v);
                if (k.Contains("REG:")) HasRegistryFiles = true;
            }

            if (jPlatform.ContainsKey("PathListToClear"))
            {
                PathListToClear = jPlatform["PathListToClear"]!.Values<string>().ToList();
                if (PathListToClear.Contains("SAME_AS_LOGIN_FILES"))
                    PathListToClear = LoginFiles.Keys.ToList();
            }

            // Variables that may not be set:
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
                        BackupPaths.Add(Basic.ExpandEnvironmentVariables(k), v);
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
                            var startMenuFiles = Directory.GetFiles(Basic.ExpandEnvironmentVariables("%StartMenuAppData%"), SafeName + ".lnk", SearchOption.AllDirectories);
                            var commonStartMenuFiles = Directory.GetFiles(Basic.ExpandEnvironmentVariables("%StartMenuProgramData%"), SafeName + ".lnk", SearchOption.AllDirectories);
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
                    var desktopShortcutFolder = Basic.ExpandEnvironmentVariables(sFolder, true);
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
        public string GetUniqueFilePath() => Basic.ExpandEnvironmentVariables(UniqueIdFile);
        public string GetShortcutImageFolder => $"img\\shortcuts\\{SafeName}\\";
        public string GetShortcutImagePath() => Path.Join(Globals.UserDataFolder, "wwwroot\\", GetShortcutImageFolder);
        public string GetShortcutIgnoredPath(string shortcut) => Path.Join(ShortcutFolder, shortcut.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));
        public string PrimaryId => PlatformIds[0];
        public string ShortcutFolder => Path.Join(PlatformLoginCache, "Shortcuts\\");
        public string PlatformLoginCache => $"LoginCache\\{SafeName}\\";
        public string IdsJsonPath => Path.Join(PlatformLoginCache, "ids.json");
        public string AccountLoginCachePath(string acc) => Path.Join(PlatformLoginCache, $"{acc}\\");
        public string GetShortcutImagePath(string gameShortcutName) =>
            Path.Join(GetShortcutImageFolder, PlatformFuncs.RemoveShortcutExt(gameShortcutName) + ".png");

        public Dictionary<string, string> ReadRegJson(string acc) =>
            GeneralFuncs.ReadDict(Path.Join(AccountLoginCachePath(acc), "reg.json"), true);

        public void SaveRegJson(Dictionary<string, string> regJson, string acc)
        {
            if (regJson.Count > 0)
                GeneralFuncs.SaveDict(regJson, Path.Join(AccountLoginCachePath(acc), "reg.json"), true);
        }

        public MarkupString GetUserModalExtraButtons => UsernameModalExtraButtons == ""
            ? new MarkupString()
            : new MarkupString(Globals.ReadAllText(Path.Join(Globals.AppDataFolder, UsernameModalExtraButtons)));

        public string GetUserModalCopyText => UserModalCopyText == "" ? "" :
            Globals.ReadAllText(Path.Join(Globals.AppDataFolder, UserModalCopyText));

        public string GetUserModalHintText()
        {
            try
            {
                return Lang[UserModalHintText];
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
    public class PlatformFuncs
    {
        public static string RemoveShortcutExt(string s) => !string.IsNullOrWhiteSpace(s) ? s.Replace(".lnk", "").Replace(".url", "") : "";
    }
}
