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

using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Classes;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.Shared.Modal;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Basic : IBasic
    {
        [Inject] private ILang Lang { get; }
        [Inject] private IAppData AppData { get; }
        [Inject] private IGeneralFuncs GeneralFuncs { get; }
        [Inject] private IAppStats AppStats { get; }
        [Inject] private IAppSettings AppSettings { get; }
        [Inject] private IAccountFuncs AccountFuncs { get; }
        [Inject] private IAppFuncs AppFuncs { get; }
        [Inject] private IModalData ModalData { get; }
        [Inject] private IBasicStats BasicStats { get; }
        [Inject] private ICurrentPlatform CurrentPlatform { get; }
        [Inject] private IShortcut Shortcut { get; }

        public Basic()
        {
            try
            {
                if (File.Exists(CurrentPlatform.SettingsFile)) JsonConvert.PopulateObject(File.ReadAllText(CurrentPlatform.SettingsFile), this);
                //_instance = JsonConvert.DeserializeObject<AppSettings>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to load BasicSettings", e);
                _ = GeneralFuncs.ShowToast("error", Lang["Toast_FailedLoadSettings"]);
                if (File.Exists(CurrentPlatform.SettingsFile))
                    Globals.CopyFile(CurrentPlatform.SettingsFile, CurrentPlatform.SettingsFile.Replace(".json", ".old.json"));
            }
            if (FolderPath.EndsWith(".exe"))
                FolderPath = Path.GetDirectoryName(FolderPath) ?? string.Join("\\", FolderPath.Split("\\")[..^1]);
            BuildContextMenu();

            DesktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.FullName);
            AppData.InitializedClasses.Basic = true;
        }

        public void SaveSettings()
        {
            // Accounts seem to reset when saving, for some reason...
            var accList = AppData.BasicAccounts;
            GeneralFuncs.SaveSettings(CurrentPlatform.SettingsFile, this);
            AppData.BasicAccounts = accList;
        }


        [JsonProperty("FolderPath", Order = 1)] private string _folderPath = "";
        public string FolderPath
        {
            get
            {
                if (!string.IsNullOrWhiteSpace(_folderPath)) return _folderPath;
                _folderPath = CurrentPlatform.DefaultFolderPath;

                return _folderPath;
            }
            set => _folderPath = value;
        }

        [JsonProperty(Order = 2)] public bool Admin { get; set; }
        [JsonProperty(Order = 3)] public int TrayAccNumber { get; set; } = 3;
        [JsonProperty(Order = 4)] public bool ForgetAccountEnabled { get; set; }
        [JsonProperty(Order = 5)] public Dictionary<int, string> Shortcuts { get; set; } = new();

        [JsonProperty("ClosingMethod", Order = 6)] private string _closingMethod = "";
        public string ClosingMethod
        {
            get
            {
                if (!string.IsNullOrWhiteSpace(_closingMethod)) return _closingMethod;
                _closingMethod = CurrentPlatform.ClosingMethod;

                return _closingMethod;
            }
            set => _closingMethod = value;
        }

        [JsonProperty("StartingMethod", Order = 7)] private string _startingMethod = "";
        public string StartingMethod {
            get
            {
                if (!string.IsNullOrWhiteSpace(_startingMethod)) return _startingMethod;
                _startingMethod = CurrentPlatform.StartingMethod;

                return _startingMethod;
            }
            set => _startingMethod = value;
        }

        [JsonProperty(Order = 8)] public bool AutoStart { get; set; } = true;

        [JsonProperty(Order = 9)] public bool ShowShortNotes { get; set; } = true;
        [JsonIgnore] public bool DesktopShortcut { get; set; }
        [JsonIgnore] public int LastAccTimestamp { get; set; }
        [JsonIgnore] public string LastAccName { get; set; }

        public ObservableCollection<MenuItem> ContextMenuItems { get; set; } = new();
        private void BuildContextMenu()
        {
            ContextMenuItems.Clear();
            ContextMenuItems.AddRange(new MenuBuilder(
                new []
                {
                    new ("Context_SwapTo", new Action(async () => await AppFuncs.SwapToAccount())),
                    new ("Context_CopyUsername", new Action(async () => await AppFuncs.CopyText(AppData.SelectedAccount.DisplayName))),
                    new ("Context_ChangeName", new Action(ModalData.ShowChangeUsernameModal)),
                    new ("Context_CreateShortcut", new Action(async () => await GeneralFuncs.CreateShortcut())),
                    new ("Context_ChangeImage", new Action(ModalData.ShowChangeAccImageModal)),
                    new ("Forget", new Action(async () => await AppFuncs.ForgetAccount())),
                    new ("Notes", new Action(() => ModalData.ShowModal("notes"))),
                    BasicStats.PlatformHasAnyGames(CurrentPlatform.SafeName) ?
                        new Tuple<string, object>("Context_ManageGameStats", "ShowGameStatsSetup(event)") : null,
                }).Result());
        }

        public ObservableCollection<MenuItem> ContextMenuShortcutItems { get; set; } = new MenuBuilder(
            new Tuple<string, object>[]
        {
            new ("Context_RunAdmin", "shortcut('admin')"),
            new ("Context_Hide", "shortcut('hide')"),
        }).Result();

        public ObservableCollection<MenuItem> ContextMenuPlatformItems { get; set; } = new MenuBuilder(
            new Tuple<string, object>("Context_RunAdmin", "shortcut('admin')")
        ).Result();

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Get exe path from BasicSettings.json
        /// </summary>
        /// <returns>exe's path string</returns>
        public string Exe() => Path.Join(FolderPath, CurrentPlatform.ExeName);

        [JSInvokable]
        public void SaveShortcutOrder(Dictionary<int, string> o)
        {
            Shortcuts = o;
            SaveSettings();
        }

        public void SetClosingMethod(string method)
        {
            ClosingMethod = method;
            SaveSettings();
        }
        public void SetStartingMethod(string method)
        {
            StartingMethod = method;
            SaveSettings();
        }
        public async Task OpenFolder(string folder)
        {
            Directory.CreateDirectory(folder); // Create if doesn't exist
            Process.Start("explorer.exe", folder);
            await GeneralFuncs.ShowToast("info", Lang["Toast_PlaceShortcutFiles"], renderTo: "toastarea");
        }

        public void RunPlatform(string exePath, bool admin, string args, string platName, string startingMethod = "Default")
        {
            _ = Globals.StartProgram(exePath, admin, args, startingMethod)
                ? GeneralFuncs.ShowToast("info", Lang["Status_StartingPlatform", new {platform = platName}], renderTo: "toastarea")
                : GeneralFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new {platform = platName}], renderTo: "toastarea");
        }


        public void RunPlatform(bool admin)
        {
            _ = Globals.StartProgram(Exe(), admin, CurrentPlatform.ExeModalData.ExtraArgs, CurrentPlatform.StartingMethod)
                ? GeneralFuncs.ShowToast("info", Lang["Status_StartingPlatform", new {platform = CurrentPlatform.SafeName}], renderTo: "toastarea")
                : GeneralFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new {platform = CurrentPlatform.SafeName}], renderTo: "toastarea");
        }
        public async Task RunPlatform()
        {
            Globals.StartProgram(Exe(), Admin, CurrentPlatform.ExeModalData.ExtraArgs, CurrentPlatform.StartingMethod);
            await GeneralFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.SafeName }], renderTo: "toastarea");
        }
        public async Task RunShortcut(string s, string shortcutFolder = "", bool admin = false, string platform = "")
        {
            AppStats.IncrementGameLaunches(platform);

            if (shortcutFolder == "")
                shortcutFolder = CurrentPlatform.ShortcutFolder;
            var proc = new Process();
            proc.StartInfo = new ProcessStartInfo
            {
                FileName = Path.GetFullPath(Path.Join(shortcutFolder, s)),
                UseShellExecute = true,
                Verb = admin ? "runas" : ""
            };

            if (s.EndsWith(".url"))
            {
                // These can not be run as admin...
                proc.StartInfo.UseShellExecute = false;
                proc.StartInfo.WindowStyle = ProcessWindowStyle.Hidden;
                proc.StartInfo.Arguments = $"/C \"{proc.StartInfo.FileName}\"";
                proc.StartInfo.FileName = "cmd.exe";
                proc.StartInfo.CreateNoWindow = true;
                proc.StartInfo.RedirectStandardInput = true;
                proc.StartInfo.RedirectStandardOutput = true;
                if (admin) await GeneralFuncs.ShowToast("warning", Lang["Toast_UrlAdminErr"], duration: 15000, renderTo: "toastarea");
            }
            else if (Globals.IsAdministrator && !admin)
            {
                proc.StartInfo.Arguments = proc.StartInfo.FileName;
                proc.StartInfo.FileName = "explorer.exe";
            }

            try
            {
                proc.Start();
                await GeneralFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = PlatformFuncs.RemoveShortcutExt(s) }], renderTo: "toastarea");
            }
            catch (Exception e)
            {
                // Cancelled by user, or another error.
                Globals.WriteToLog($"Tried to start \"{s}\" but failed.", e);
                await GeneralFuncs.ShowToast("error", Lang["Status_FailedLog"], duration: 15000, renderTo: "toastarea");
            }
        }

        [JSInvokable]
        public async Task HandleShortcutAction(string shortcut, string action)
        {
            if (shortcut == "btnStartPlat") // Start platform requested
            {
                RunPlatform(action == "admin");
                return;
            }

            if (!Shortcuts.ContainsValue(shortcut)) return;

            switch (action)
            {
                case "hide":
                {
                    // Remove shortcut from folder, and list.
                    Shortcuts.Remove(Shortcuts.First(e => e.Value == shortcut).Key);
                    var f = Path.Join(CurrentPlatform.ShortcutFolder, shortcut);
                    if (File.Exists(f)) File.Move(f, f.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));

                    // Save.
                    SaveSettings();
                    break;
                }
                case "admin":
                    await RunShortcut(shortcut, admin: true);
                    break;
            }
        }

        #region SETTINGS
        /// <summary>
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\ResetSettings]");
            FolderPath = CurrentPlatform.DefaultFolderPath;
            Admin = false;
            TrayAccNumber = 3;
            DesktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.FullName);

            SaveSettings();
        }
        #endregion



        /// <summary>
        /// Main function for Basic Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public async Task LoadProfiles()
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.LoadProfiles] Loading Basic profiles for: " + CurrentPlatform.FullName);
            await GeneralFuncs.GenericLoadAccounts(CurrentPlatform.FullName, true);
        }

        #region Account IDs

        public Dictionary<string, string> AccountIds;
        public void LoadAccountIds() => AccountIds = GeneralFuncs.ReadDict(CurrentPlatform.IdsJsonPath);
        //public static void LoadAccountIds()
        //{
        //    var p = Path.GetFullPath(CurrentPlatform.IdsJsonPath);
        //    AccountIds = GeneralFuncs.ReadDict(p);
        //    return;
        //}
        private void SaveAccountIds() =>
            File.WriteAllText(CurrentPlatform.IdsJsonPath, JsonConvert.SerializeObject(AccountIds));
        public string GetNameFromId(string accId) => AccountIds.ContainsKey(accId) ? AccountIds[accId] : accId;
        #endregion

        /// <summary>
        /// Restart Basic with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accId">(Optional) User's unique account ID</param>
        /// <param name="args">Starting arguments</param>
        [SupportedOSPlatform("windows")]
        public async void SwapBasicAccounts(string accId = "", string args = "")
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.SwapBasicAccounts] Swapping to: hidden.");
            // Handle args:
            if (CurrentPlatform.ExeModalData.ExtraArgs != "")
            {
                args = CurrentPlatform.ExeModalData.ExtraArgs + (args == "" ? "" : " " + args);
            }

            LoadAccountIds();
            var accName = GetNameFromId(accId);

            if (!await KillGameProcesses())
                return;

            // Add currently logged in account if there is a way of checking unique ID.
            // If saved, and has unique key: Update
            if (CurrentPlatform.UniqueIdFile is not null)
            {
                var uniqueId = await GetUniqueId();

                // UniqueId Found >> Save!
                if (File.Exists(CurrentPlatform.IdsJsonPath))
                {
                    if (!string.IsNullOrEmpty(uniqueId) && AccountIds.ContainsKey(uniqueId))
                    {
                        if (accId == uniqueId)
                        {
                            await GeneralFuncs.ShowToast("info", Lang["Toast_AlreadyLoggedIn"], renderTo: "toastarea");
                            if (AutoStart)
                            {
                                _ = Globals.StartProgram(Exe(), Admin, args,
                                    CurrentPlatform.StartingMethod)
                                    ? GeneralFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.SafeName }], renderTo: "toastarea")
                                    : GeneralFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new { platform = CurrentPlatform.SafeName }], renderTo: "toastarea");
                            }
                            await AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);

                            return;
                        }
                        await BasicAddCurrent(AccountIds[uniqueId]);
                    }
                }
            }

            // Clear current login
            ClearCurrentLoginBasic();

            // Copy saved files in
            if (accName != "")
            {
                if (!await BasicCopyInAccount(accId)) return;
                Globals.AddTrayUser(CurrentPlatform.SafeName, $"+{CurrentPlatform.PrimaryId}:" + accId, accName, TrayAccNumber); // Add to Tray list, using first Identifier
            }

            if (AutoStart)
                RunPlatform(Exe(), Admin, args, CurrentPlatform.FullName, CurrentPlatform.StartingMethod);

            if (accName != "" && AutoStart && AppSettings.MinimizeOnSwitch) await AppData.InvokeVoidAsync("hideWindow");

            NativeFuncs.RefreshTrayArea();
            await AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
            AppStats.IncrementSwitches(CurrentPlatform.SafeName);

            try
            {
                LastAccName = accId;
                LastAccTimestamp = Globals.GetUnixTimeInt();
                if (LastAccName != "")
                    await AccountFuncs.SetCurrentAccount(LastAccName);
            }
            catch (Exception)
            {
                //
            }
        }

        public async Task<string> GetCurrentAccountId()
        {
            // 30 second window - For when changing accounts
            if (LastAccName != "" && LastAccTimestamp - Globals.GetUnixTimeInt() < 30)
                return LastAccName;

            try
            {
                var uniqueId = await GetUniqueId();

                // UniqueId Found in saved file >> return value
                if (File.Exists(CurrentPlatform.IdsJsonPath) && !string.IsNullOrEmpty(uniqueId) && AccountIds.ContainsKey(uniqueId))
                    return uniqueId;
            }
            catch (Exception)
            {
                //
            }

            return "";
        }


        [SupportedOSPlatform("windows")]
        private void ClearCurrentLoginBasic()
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.ClearCurrentLoginBasic]");

            // Foreach file/folder/reg in Platform.PathListToClear
            if (CurrentPlatform.PathListToClear.Any(accFile => !DeleteFileOrFolder(accFile).Result)) return;

            if (CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
            {
                var path = CurrentPlatform.GetUniqueFilePath().Split("::")[0];
                var selector = CurrentPlatform.GetUniqueFilePath().Split("::")[1];
                Globals.ReplaceVarInJsonFile(path, selector, "");
            }

            if (CurrentPlatform.UniqueIdMethod != "CREATE_ID_FILE") return;

            // Unique ID file --> This needs to be deleted for a new instance
            var uniqueIdFile = CurrentPlatform.GetUniqueFilePath();
            Globals.DeleteFile(uniqueIdFile);
        }

        public async Task ClearCache()
        {

            Globals.DebugWriteLine(@"[Func:Basic\Basic.ClearCache]");
            var totalFiles = 0;
            var totalSize = Globals.FileLengthToString(CurrentPlatform.CachePaths.Sum(x => SizeOfFile(x, ref totalFiles)));
            await GeneralFuncs.ShowToast("info", Lang["Platform_ClearCacheTotal", new { totalFileCount = totalFiles, totalSizeMB = totalSize }], Lang["Working"], "toastarea");

            // Foreach file/folder/reg in Platform.PathListToClear
            foreach (var f in CurrentPlatform.CachePaths.Where(f => !DeleteFileOrFolder(f).Result))
            {
                await GeneralFuncs.ShowToast("error", Lang["Platform_CouldNotDeleteLog", new { logPath = Globals.GetLogPath() }], Lang["Working"], "toastarea");
                Globals.WriteToLog("Could not delete: " + f);
            }

            await GeneralFuncs.ShowToast("success", Lang["DeletedFiles"], Lang["Working"], "toastarea");
        }

        private long SizeOfFile(string accFile, ref int numFiles)
        {
            // The "file" is a registry key
            if (accFile.StartsWith("REG:"))
                return 0;

            long totalSize = 0;
            numFiles = 0;

            // Handle wildcards
            DirectoryInfo di;
            if (accFile.Contains('*'))
            {
                var folder = ExpandEnvironmentVariables(Path.GetDirectoryName(accFile) ?? "");
                var file = Path.GetFileName(accFile);
                di = new DirectoryInfo(folder);

                var so = SearchOption.TopDirectoryOnly;
                var searchPattern = file;
                // "...\\*" is recursive
                if (file == "*")
                {
                    searchPattern = "*";
                    so = SearchOption.AllDirectories;
                }

                // while "...\\*.log" or "...\\file_*" are not.
                foreach (var fi in di.EnumerateFiles(searchPattern, so))
                {
                    totalSize += fi.Length;
                    numFiles++;
                }

                return totalSize;
            }

            var fullPath = ExpandEnvironmentVariables(accFile);
            // Is folder? Recursive get file size
            if (Directory.Exists(fullPath))
            {
                di = new DirectoryInfo(fullPath);
                foreach (var fi in di.EnumerateFiles("*", SearchOption.AllDirectories))
                {
                    totalSize += fi.Length;
                    numFiles++;
                }

                return totalSize;
            }

            // Is file? Get file size
            if (!File.Exists(fullPath)) return 0;
            numFiles++;
            return new FileInfo(fullPath).Length;
        }

        private async Task<bool> DeleteFileOrFolder(string accFile)
        {
            // The "file" is a registry key
            if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
            {
                // If set to clear LoginCache for account before adding (Enabled by default):
                if (CurrentPlatform.RegDeleteOnClear)
                {
                    if (Globals.DeleteRegistryKey(accFile[4..])) return true;
                }
                else
                {
                    if (Globals.SetRegistryKey(accFile[4..])) return true;
                }
                await GeneralFuncs.ShowToast("error", Lang["Toast_RegFailWrite"], Lang["Error"], "toastarea");
                return false;
            }

            // The "file" is a JSON value
            if (accFile.StartsWith("JSON"))
            {
                if (accFile.StartsWith("JSON_SELECT"))
                {
                    var path = accFile.Split("::")[1];
                    var selector = accFile.Split("::")[2];
                    Globals.ReplaceVarInJsonFile(path, selector, "");
                }
            }

            // Handle wildcards
            if (accFile.Contains('*'))
            {
                var folder = ExpandEnvironmentVariables(Path.GetDirectoryName(accFile) ?? "");
                var file = Path.GetFileName(accFile);

                // Handle "...\\*" folder.
                if (file == "*")
                {
                    if (!Directory.Exists(Path.GetDirectoryName(folder)))
                        return true;
                    if (!Globals.RecursiveDelete(folder, false))
                        await GeneralFuncs.ShowToast("error", Lang["Platform_DeleteFail"], Lang["Error"], "toastarea");
                    return true;
                }

                // Handle "...\\*.log" or "...\\file_*", etc.
                // This is NOT recursive - Specify folders manually in JSON
                if (!Directory.Exists(folder)) return true;
                foreach (var f in Directory.GetFiles(folder, file))
                    Globals.DeleteFile(f);

                return true;
            }

            var fullPath = ExpandEnvironmentVariables(accFile);
            // Is folder? Recursive copy folder
            if (Directory.Exists(fullPath))
            {
                if (!Globals.RecursiveDelete(fullPath, true))
                    await GeneralFuncs.ShowToast("error", Lang["Platform_DeleteFail"], Lang["Error"], "toastarea");
                return true;
            }

            try
            {
                // Is file? Delete file
                Globals.DeleteFile(fullPath, true);
            }
            catch (UnauthorizedAccessException e)
            {
                Globals.WriteToLog(e);
                await GeneralFuncs.ShowToast("error", Lang["Platform_DeleteFail"], Lang["Error"], "toastarea");
            }
            return true;
        }

        /// <summary>
        /// Get string contents of registry file, or path to file matching regex/wildcards.
        /// </summary>
        /// <param name="accFile"></param>
        /// <param name="regex"></param>
        /// <returns></returns>
        private string RegexSearchFileOrFolder(string accFile, string regex)
        {
            accFile = ExpandEnvironmentVariables(accFile);
            regex = Globals.ExpandRegex(regex);
            // The "file" is a registry key
            if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
            {
                var res = Globals.ReadRegistryKey(accFile);
                switch (res)
                {
                    case string:
                        return res;
                    case byte[] bytes:
                        return Globals.GetSha256HashString(bytes);
                    default:
                        Globals.WriteToLog($"REG was read, and was returned something that is not a string or byte array! {accFile}.");
                        Globals.WriteToLog("Check to see what is expected here and report to TechNobo.");
                        return res;
                }
            }


            // Handle wildcards
            if (accFile.Contains('*'))
            {
                var folder = ExpandEnvironmentVariables(Path.GetDirectoryName(accFile) ?? "");
                var file = Path.GetFileName(accFile);

                // Handle "...\\*" folder.
                // as well as "...\\*.log" or "...\\file_*", etc.
                // This is NOT recursive - Specify folders manually in JSON
                return Directory.Exists(folder) ? Globals.RegexSearchFolder(folder, regex, file) : "";
            }

            var fullPath = ExpandEnvironmentVariables(accFile);
            // Is folder? Search folder.
            if (Directory.Exists(fullPath))
                return Globals.RegexSearchFolder(fullPath, regex);

            // Is file? Search file
            var m = Regex.Match(File.ReadAllText(fullPath!), regex);
            return m.Success ? m.Value : "";
        }

        /// <summary>
        /// Expands custom environment variables.
        /// </summary>
        /// <param name="path"></param>
        /// <param name="noIncludeBasicCheck">Whether to skip initializing BasicSettings - Useful for Steam and other hardcoded platforms</param>
        /// <returns></returns>
        public string ExpandEnvironmentVariables(string path, bool noIncludeBasicCheck = false)
        {
            path = Globals.ExpandEnvironmentVariables(path);

            if (!noIncludeBasicCheck)
                path = path.Replace("%Platform_Folder%", FolderPath ?? "");

            return Environment.ExpandEnvironmentVariables(path);
        }

        private async Task<bool> BasicCopyInAccount(string accId)
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.BasicCopyInAccount]");
            LoadAccountIds();
            var accName = GetNameFromId(accId);

            var localCachePath = CurrentPlatform.AccountLoginCachePath(accName);
            _ = Directory.CreateDirectory(localCachePath);

            if (CurrentPlatform.LoginFiles == null) throw new Exception("No data in basic platform: " + CurrentPlatform.FullName);

            // Get unique ID from IDs file if unique ID is a registry key. Set if exists.
            if (OperatingSystem.IsWindows() && CurrentPlatform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(CurrentPlatform.UniqueIdFile))
            {
                var uniqueId = GeneralFuncs.ReadDict(CurrentPlatform.SafeName).FirstOrDefault(x => x.Value == accName).Key;

                if (!string.IsNullOrEmpty(uniqueId) && !Globals.SetRegistryKey(CurrentPlatform.UniqueIdFile, uniqueId)) // Remove "REG:" and read data
                {
                    await GeneralFuncs.ShowToast("error", Lang["Toast_AlreadyLoggedIn"], Lang["Error"], "toastarea");
                    return false;
                }
            }

            var regJson = CurrentPlatform.HasRegistryFiles ? CurrentPlatform.ReadRegJson(accName) : new Dictionary<string, string>();

            foreach (var (accFile, savedFile) in CurrentPlatform.LoginFiles)
            {
                // The "file" is a registry key
                if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
                {
                    if (!regJson.ContainsKey(accFile))
                    {
                        await GeneralFuncs.ShowToast("error", Lang["Toast_RegFailReadSaved"], Lang["Error"], "toastarea");
                        continue;
                    }

                    var regValue = regJson[accFile] ?? "";

                    if (!Globals.SetRegistryKey(accFile[4..], regValue)) // Remove "REG:" and read data
                    {
                        await GeneralFuncs.ShowToast("error", Lang["Toast_RegFailWrite"], Lang["Error"], "toastarea");
                        return false;
                    }
                    continue;
                }

                // The "file" is a JSON value
                if (accFile.StartsWith("JSON"))
                {
                    var jToken = GeneralFuncs.ReadJsonFile(Path.Join(localCachePath, savedFile));

                    var path = accFile.Split("::")[1];
                    var selector = accFile.Split("::")[2];
                    if (!Globals.ReplaceVarInJsonFile(path, selector, jToken))
                    {
                        await GeneralFuncs.ShowToast("error", Lang["Toast_JsonFailModify"], Lang["Error"], "toastarea");
                        return false;
                    }
                    continue;
                }


                // FILE OR FOLDER
                await HandleFileOrFolder(accFile, savedFile, localCachePath, true);
            }

            return true;
        }

        private async Task<bool> KillGameProcesses()
        {
            // Kill game processes
            await AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatform", new { platform = CurrentPlatform.FullName }]);
            if (await GeneralFuncs.CloseProcesses(CurrentPlatform.ExesToEnd, ClosingMethod)) return true;

            if (Globals.IsAdministrator)
                await AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = CurrentPlatform.FullName }]);
            else
            {
                await GeneralFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                ModalData.ShowModal("confirm", ExtraArg.RestartAsAdmin);
            }
            return false;

        }

        [SupportedOSPlatform("windows")]
        public async Task<bool> BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.BasicAddCurrent]");
            if (CurrentPlatform.ExitBeforeInteract)
                if (!await KillGameProcesses())
                    return false;

            // If set to clear LoginCache for account before adding (Enabled by default):
            if (CurrentPlatform.ClearLoginCache)
            {
                Globals.RecursiveDelete(CurrentPlatform.AccountLoginCachePath(accName), false);
            }

            // Separate special arguments (if any)
            var specialString = "";
            if (CurrentPlatform.HasExtras && accName.Contains(":{"))
            {
                var index = accName.IndexOf(":{", StringComparison.Ordinal)! + 1;
                specialString = accName[index..];
                accName = accName.Split(":{")[0];
            }

            var localCachePath = CurrentPlatform.AccountLoginCachePath(accName);
            _ = Directory.CreateDirectory(localCachePath);

            if (CurrentPlatform.LoginFiles == null) throw new Exception("No data in basic platform: " + CurrentPlatform.FullName);

            // Handle unique ID
            await AppData.InvokeVoidAsync("updateStatus", Lang["Status_GetUniqueId"]);

            var uniqueId = await GetUniqueId();

            if (uniqueId == "" && CurrentPlatform.UniqueIdMethod == "CREATE_ID_FILE")
            {
                // Unique ID file, and does not already exist: Therefore create!
                var uniqueIdFile = CurrentPlatform.GetUniqueFilePath();
                uniqueId = Globals.RandomString(16);
                await File.WriteAllTextAsync(uniqueIdFile, uniqueId);
            }

            // Handle special args in username
            var hadSpecialProperties = ProcessSpecialAccName(specialString, accName, uniqueId);

            var regJson = CurrentPlatform.HasRegistryFiles ? CurrentPlatform.ReadRegJson(accName) : new Dictionary<string, string>();

            await AppData.InvokeVoidAsync("updateStatus", Lang["Status_CopyingFiles"]);
            foreach (var (accFile, savedFile) in CurrentPlatform.LoginFiles)
            {
                // HANDLE REGISTRY KEY
                if (accFile.StartsWith("REG:"))
                {
                    var trimmedName = accFile[4..];

                    if (ReadRegistryKeyWithErrors(trimmedName, out var response)) // Remove "REG:                    " and read data
                    {
                        switch (response)
                        {
                            // Write registry value to provided file
                            case string s:
                                regJson[accFile] = s;
                                break;
                            case byte[] ba:
                                regJson[accFile] = "(hex) " + Globals.ByteArrayToString(ba);
                                break;
                            default:
                                Globals.WriteToLog($"Unexpected registry type encountered (2)! Report to TechNobo. {response.GetType()}");
                                break;
                        }
                    }
                    continue;
                }

                // HANDLE JSON VALUE
                if (accFile.StartsWith("JSON"))
                {
                    var path = accFile.Split("::")[1];
                    var selector = accFile.Split("::")[2];
                    var js = await GeneralFuncs.ReadJsonFile(path);
                    var originalValue = js.SelectToken(selector);

                    // Save if it's JUST getting the value
                    if (!accFile.StartsWith("JSON_SELECT_"))
                        Globals.SaveJsonFile(Path.Join(localCachePath, savedFile), originalValue);

                    // Otherwise, if it's selecting a part of it
                    string delimiter;
                    var firstResult = true;
                    if (accFile.StartsWith("JSON_SELECT_FIRST"))
                    {
                        delimiter = accFile.Split("JSON_SELECT_FIRST")[1].Split("::")[0];
                    }
                    else
                    {
                        delimiter = accFile.Split("JSON_SELECT_LAST")[1].Split("::")[0];
                        firstResult = false;
                    }

                    var originalValueString = (string)originalValue;
                    originalValueString = Globals.GetCleanFilePath(firstResult ? originalValueString.Split(delimiter).First() : originalValueString.Split(delimiter).Last());

                    Globals.SaveJsonFile(Path.Join(localCachePath, savedFile), originalValueString);
                    continue;
                }

                // FILE OR FOLDER
                if (await HandleFileOrFolder(accFile, savedFile, localCachePath, false)) continue;

                // Could not find file/folder
                await GeneralFuncs.ShowToast("error", Lang["CouldNotFindX", new { x = accFile }], Lang["DirectoryNotFound"], "toastarea");
                return false;

                // TODO: Run some action that can be specified in the Platforms.json file
                // Add for the start, and end of this function -- To allow 'plugins'?
                // Use reflection?
            }

            CurrentPlatform.SaveRegJson(regJson, accName);

            var allIds = GeneralFuncs.ReadDict(CurrentPlatform.IdsJsonPath);
            allIds[uniqueId] = accName;
            await File.WriteAllTextAsync(CurrentPlatform.IdsJsonPath, JsonConvert.SerializeObject(allIds));

            // Copy in profile image from default -- As long as not already handled by special arguments
            // Or if has ProfilePicFromFile and ProfilePicRegex.
            if (!hadSpecialProperties.Contains("IMAGE|"))
            {
                await AppData.InvokeVoidAsync("updateStatus", Lang["Status_HandlingImage"]);

                _ = Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{CurrentPlatform.SafeName}"));
                var profileImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{CurrentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg");
                if (!File.Exists(profileImg))
                {
                    var platformImgPath = "\\img\\platform\\" + CurrentPlatform.SafeName + "Default.png";

                    // Copy in profile picture (if found) from Regex search of files (if defined)
                    if (CurrentPlatform.ProfilePicFromFile != "" && CurrentPlatform.ProfilePicRegex != "")
                    {
                        var res = Globals.GetCleanFilePath(RegexSearchFileOrFolder(CurrentPlatform.ProfilePicFromFile, CurrentPlatform.ProfilePicRegex));
                        var sourcePath = res;
                        if (CurrentPlatform.ProfilePicPath != "")
                        {
                            // The regex result should be considered a filename.
                            // Sub in %FileName% from res, and %UniqueId% from uniqueId
                            sourcePath = ExpandEnvironmentVariables(CurrentPlatform.ProfilePicPath.Replace("%FileName%", res).Replace("%UniqueId", uniqueId));
                        }

                        if (res != "" && File.Exists(sourcePath))
                            if (!Globals.CopyFile(sourcePath, profileImg))
                                Globals.WriteToLog("Tried to save profile picture from path (ProfilePicFromFile, ProfilePicRegex method)");
                    }
                    else if (CurrentPlatform.ProfilePicPath != "")
                    {
                        var sourcePath = ExpandEnvironmentVariables(Globals.GetCleanFilePath(CurrentPlatform.ProfilePicPath.Replace("%UniqueId", uniqueId))) ?? "";
                        if (sourcePath != "" && File.Exists(sourcePath))
                            if (!Globals.CopyFile(sourcePath, profileImg))
                                Globals.WriteToLog("Tried to save profile picture from path (ProfilePicPath method)");
                    }

                    // Else (If file couldn't be saved, or not found -> Default.
                    if (!File.Exists(profileImg))
                    {
                        var currentPlatformImgPath = Path.Join(GeneralFuncs.WwwRoot(), platformImgPath);
                        Globals.CopyFile(File.Exists(currentPlatformImgPath)
                            ? Path.Join(currentPlatformImgPath)
                            : Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png"), profileImg);
                    }
                }
            }

            AppData.NavigateTo(
                $"/Basic/?cacheReload&toast_type=success&toast_title={Uri.EscapeDataString(Lang["Success"])}&toast_message={Uri.EscapeDataString(Lang["Toast_SavedItem", new { item = accName }])}", true);
            return true;
        }

        /// <summary>
        /// Handles copying files or folders around
        /// </summary>
        /// <param name="fromPath"></param>
        /// <param name="toPath"></param>
        /// <param name="localCachePath"></param>
        /// <param name="reverse">FALSE: Platform -> LoginCache. TRUE: LoginCache -> J••Platform</param>
        private async Task<bool> HandleFileOrFolder(string fromPath, string toPath, string localCachePath, bool reverse)
        {
            // Expand, or join localCachePath
            var toFullPath = toPath.Contains('%')
                ? ExpandEnvironmentVariables(toPath)
                : Path.Join(localCachePath, toPath);

            // Reverse if necessary. Explained in summary above.
            if (reverse && fromPath.Contains('*'))
            {
                (toPath, fromPath) = (fromPath, toPath); // Reverse
                var wildcard = Path.GetFileName(toPath);
                // Expand, or join localCachePath
                fromPath = fromPath.Contains('%')
                    ? ExpandEnvironmentVariables(Path.Join(fromPath, wildcard))
                    : Path.Join(localCachePath, fromPath, wildcard);
                toPath = toPath.Replace(wildcard, "");
                toFullPath = toPath;
            }

            // Handle wildcards
            if (fromPath.Contains('*'))
            {
                var folder = ExpandEnvironmentVariables(Path.GetDirectoryName(fromPath) ?? "");
                var file = Path.GetFileName(fromPath);

                // Handle "...\\*" folder.
                if (file == "*")
                {
                    if (!Directory.Exists(Path.GetDirectoryName(fromPath))) return false;
                    if (Globals.CopyFilesRecursive(Path.GetDirectoryName(fromPath), toFullPath)) return true;

                    await GeneralFuncs.ShowToast("error", Lang["Toast_FileCopyFail"], renderTo: "toastarea");
                    return false;
                }

                // Handle "...\\*.log" or "...\\file_*", etc.
                // This is NOT recursive - Specify folders manually in JSON
                _ = Directory.CreateDirectory(folder);
                foreach (var f in Directory.GetFiles(folder, file))
                {
                    if (toFullPath == null) return false;
                    if (toFullPath.Contains('*')) toFullPath = Path.GetDirectoryName(toFullPath);
                    var fullOutputPath = Path.Join(toFullPath, Path.GetFileName(f));
                    Globals.CopyFile(f, fullOutputPath);
                }

                return true;
            }

            if (reverse)
                (fromPath, toFullPath) = (toFullPath, fromPath);

            var fullPath = ExpandEnvironmentVariables(fromPath);
            // Is folder? Recursive copy folder
            if (Directory.Exists(fullPath))
            {
                _ = Directory.CreateDirectory(toFullPath);
                if (Globals.CopyFilesRecursive(fullPath, toFullPath)) return true;
                await GeneralFuncs.ShowToast("error", Lang["Toast_FileCopyFail"], renderTo: "toastarea");
                return false;
            }

            // Is file? Copy file
            if (!File.Exists(fullPath)) return false;
            _ = Directory.CreateDirectory(Path.GetDirectoryName(toFullPath) ?? string.Empty);
            var dest = Path.Join(Path.GetDirectoryName(toFullPath), Path.GetFileName(fullPath));
            Globals.CopyFile(fullPath, dest);
            return true;

        }

        /// <summary>
        /// Do special actions with AccName, and return cleaned AccName when done.
        /// </summary>
        /// <param name="accName">Account Name:{JSON OBJECT}</param>
        /// <param name="uniqueId">Unique ID of account</param>
        /// <param name="jsonString">JSON string of actions to perform on account</param>
        private string ProcessSpecialAccName(string jsonString, string accName, string uniqueId)
        {
            // Verify existence of possible extra properties
            var hadSpecialProperties = "";
            if (!CurrentPlatform.HasExtras) return hadSpecialProperties;
            var specialProperties = JsonConvert.DeserializeObject<Dictionary<string, string>>(jsonString);
            if (specialProperties == null) return hadSpecialProperties;

            // HANDLE SPECIAL IMAGE
            var profileImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{CurrentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg");
            if (specialProperties.ContainsKey("image"))
            {
                var imageIsUrl = Uri.TryCreate(specialProperties["image"], UriKind.Absolute, out var uriResult)
                                 && (uriResult.Scheme == Uri.UriSchemeHttp || uriResult.Scheme == Uri.UriSchemeHttps);
                if (imageIsUrl)
                {
                    // Is url -> Download
                    if (Globals.DownloadFile(specialProperties["image"], profileImg))
                        hadSpecialProperties = "IMAGE|";
                }
                else
                {
                    // Is not url -> Copy file
                    if (Globals.CopyFile(ExpandEnvironmentVariables(specialProperties["image"]), profileImg))
                        hadSpecialProperties = "IMAGE|";
                }
            }

            return hadSpecialProperties;
        }

        public async Task<string> GetUniqueId()
        {
            if (OperatingSystem.IsWindows() && CurrentPlatform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(CurrentPlatform.UniqueIdFile))
            {
                if (!ReadRegistryKeyWithErrors(CurrentPlatform.UniqueIdFile, out var r))
                    return "";

                switch (r)
                {
                    case string s:
                        return s;
                    case byte[]:
                        return Globals.GetSha256HashString(r);
                    default:
                        Globals.WriteToLog($"Unexpected registry type encountered (1)! Report to TechNobo. {r.GetType()}");
                        return "";
                }
            }

            var fileToRead = CurrentPlatform.GetUniqueFilePath();
            var uniqueId = "";

            if (CurrentPlatform.UniqueIdMethod is "CREATE_ID_FILE")
            {
                return File.Exists(fileToRead) ? await File.ReadAllTextAsync(fileToRead) : uniqueId;
            }

            if (uniqueId == "" && CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
            {
                JToken js;
                string searchFor;
                if (uniqueId == "" && CurrentPlatform.UniqueIdMethod == "JSON_SELECT")
                {
                    js = await GeneralFuncs.ReadJsonFile(CurrentPlatform.GetUniqueFilePath().Split("::")[0]);
                    searchFor = CurrentPlatform.GetUniqueFilePath().Split("::")[1];
                    uniqueId = Globals.GetCleanFilePath((string)js.SelectToken(searchFor));
                    return uniqueId;
                }

                string delimiter;
                var firstResult = true;
                if (CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT_FIRST"))
                {
                    delimiter = CurrentPlatform.UniqueIdMethod.Split("JSON_SELECT_FIRST")[1];
                }
                else
                {
                    delimiter = CurrentPlatform.UniqueIdMethod.Split("JSON_SELECT_LAST")[1];
                    firstResult = false;
                }

                js = await GeneralFuncs.ReadJsonFile(CurrentPlatform.GetUniqueFilePath().Split("::")[0]);
                searchFor = CurrentPlatform.GetUniqueFilePath().Split("::")[1];
                var res = (string)js.SelectToken(searchFor);
                if (res is null)
                    return "";
                uniqueId = Globals.GetCleanFilePath(firstResult ? res.Split(delimiter).First() : res.Split(delimiter).Last());
                return uniqueId;
            }

            if (fileToRead != null && CurrentPlatform.UniqueIdFile is not "" && (File.Exists(fileToRead) || fileToRead.Contains('*')))
            {
                if (!string.IsNullOrEmpty(CurrentPlatform.UniqueIdRegex))
                {
                    uniqueId = Globals.GetCleanFilePath(RegexSearchFileOrFolder(fileToRead, CurrentPlatform.UniqueIdRegex)); // Get unique ID from Regex, but replace any illegal characters.
                }
                else if (CurrentPlatform.UniqueIdMethod is "FILE_MD5") // TODO: TEST THIS! -- This is used for static files that do not change throughout the lifetime of an account login.
                {
                    uniqueId = GeneralFuncs.GetFileMd5(fileToRead.Contains('*')
                        ? Directory.GetFiles(Path.GetDirectoryName(fileToRead) ?? string.Empty, Path.GetFileName(fileToRead)).First()
                        : fileToRead);
                }
            }
            else if (uniqueId != "")
                uniqueId = Globals.GetSha256HashString(uniqueId);

            return uniqueId;
        }

        [SupportedOSPlatform("windows")]
        private bool ReadRegistryKeyWithErrors(string key, out dynamic value)
        {
            value = Globals.ReadRegistryKey(key);
            switch (value)
            {
                case "ERROR-NULL":
                    _ = GeneralFuncs.ShowToast("error", Lang["Toast_AccountIdReg"], Lang["Error"], "toastarea");
                    return false;
                case "ERROR-READ":
                    _ = GeneralFuncs.ShowToast("error", Lang["Toast_RegFailRead"], Lang["Error"], "toastarea");
                    return false;
            }

            return true;
        }

        public async Task ChangeUsername(string accId, string newName, bool reload = true)
        {
            LoadAccountIds();
            var oldName = GetNameFromId(accId);

            try
            {
                // No need to rename image as accId. That step is skipped here.
                Directory.Move($"LoginCache\\{CurrentPlatform.SafeName}\\{oldName}\\", $"LoginCache\\{CurrentPlatform.SafeName}\\{newName}\\"); // Rename login cache folder
            }
            catch (IOException e)
            {
                Globals.WriteToLog("Failed to write to file: " + e);
                await GeneralFuncs.ShowToast("error", Lang["Error_FileAccessDenied", new { logPath = Globals.GetLogPath() }], Lang["Error"], renderTo: "toastarea");
                return;
            }

            try
            {
                AccountIds[accId] = newName;
                SaveAccountIds();
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to change username: " + e);
                await GeneralFuncs.ShowToast("error", Lang["Toast_CantChangeUsername"], Lang["Error"], renderTo: "toastarea");
                return;
            }

            if (AppData.SelectedAccount is not null)
            {
                AppData.SelectedAccount.DisplayName = ModalData.TextInput.LastString;
                AppData.SelectedAccount.NotifyDataChanged();
            }

            await GeneralFuncs.ShowToast("success", Lang["Toast_ChangedUsername"], renderTo: "toastarea");
        }

        public Dictionary<string, string> ReadAllIds(string path = null)
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.ReadAllIds]");
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            path ??= CurrentPlatform.IdsJsonPath;
            if (!File.Exists(path)) return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
            try
            {
                s = Globals.ReadAllText(path);
            }
            catch (Exception)
            {
                //
            }

            return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }
    }
}
