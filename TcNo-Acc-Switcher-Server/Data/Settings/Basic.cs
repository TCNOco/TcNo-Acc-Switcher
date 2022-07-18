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

using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Classes;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public interface IBasic
    {
        void Init();
        void SaveSettings();
        string FolderPath { get; set; }
        bool Admin { get; set; }
        int TrayAccNumber { get; set; }
        bool ForgetAccountEnabled { get; set; }
        Dictionary<int, string> Shortcuts { get; set; }
        string ClosingMethod { get; set; }
        string StartingMethod { get; set; }
        bool AutoStart { get; set; }
        bool ShowShortNotes { get; set; }
        bool DesktopShortcut { get; set; }
        int LastAccTimestamp { get; set; }
        string LastAccName { get; set; }
        ObservableCollection<MenuItem> ContextMenuItems { get; set; }
        ObservableCollection<MenuItem> ContextMenuShortcutItems { get; set; }
        ObservableCollection<MenuItem> ContextMenuPlatformItems { get; set; }

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        void SetForgetAcc(bool enabled);

        /// <summary>
        /// Get exe path from BasicSettings.json
        /// </summary>
        /// <returns>exe's path string</returns>
        string Exe();

        void SaveShortcutOrder(Dictionary<int, string> o);
        void SetClosingMethod(string method);
        void SetStartingMethod(string method);
        void RunPlatform(bool admin);
        Task RunPlatform();
        Task HandleShortcutAction(string shortcut, string action);

        /// <summary>
        /// </summary>
        void ResetSettings();

        void LoadAccountIds();
        string GetNameFromId(string accId);

        /// <summary>
        /// Restart Basic with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accId">(Optional) User's unique account ID</param>
        /// <param name="args">Starting arguments</param>
        void SwapBasicAccounts(string accId = "", string args = "");

        Task<string> GetCurrentAccountId();
        Task<bool> BasicAddCurrent(string accName);
        Task ChangeUsername(string accId, string newName, bool reload = true);
    }

    public class Basic : IBasic
    {
        private readonly ILang _lang;
        private readonly IAppData _appData;
        private readonly IGeneralFuncs _generalFuncs;
        private readonly IAppStats _appStats;
        private readonly IAppSettings _appSettings;
        private readonly IAccountFuncs _accountFuncs;
        private readonly IAppFuncs _appFuncs;
        private readonly IModalData _modalData;
        private readonly IBasicStats _basicStats;
        private readonly ICurrentPlatform _currentPlatform;

        public Basic(ILang lang, IAppData appData, IGeneralFuncs generalFuncs, IAppStats appStats,
            IAppSettings appSettings, IAccountFuncs accountFuncs, IAppFuncs appFuncs, IModalData modalData,
            IBasicStats basicStats, ICurrentPlatform currentPlatform)
        {
            _lang = lang;
            _appData = appData;
            _generalFuncs = generalFuncs;
            _appStats = appStats;
            _appSettings = appSettings;
            _accountFuncs = accountFuncs;
            _appFuncs = appFuncs;
            _modalData = modalData;
            _basicStats = basicStats;
            _currentPlatform = currentPlatform;

            if (_currentPlatform.SettingsFile is null) return;
            Init();
        }

        private bool _isInit;

        public void Init()
        {
            if (_isInit) return;
            _isInit = true;
            try
            {
                if (File.Exists(_currentPlatform.SettingsFile))
                    JsonConvert.PopulateObject(File.ReadAllText(_currentPlatform.SettingsFile), this);
                //_instance = JsonConvert.DeserializeObject<AppSettings>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to load BasicSettings", e);
                _ = _generalFuncs.ShowToast("error", _lang["Toast_FailedLoadSettings"]);
                if (File.Exists(_currentPlatform.SettingsFile))
                    Globals.CopyFile(_currentPlatform.SettingsFile,
                        _currentPlatform.SettingsFile.Replace(".json", ".old.json"));
            }

            if (FolderPath.EndsWith(".exe"))
                FolderPath = Path.GetDirectoryName(FolderPath) ?? string.Join("\\", FolderPath.Split("\\")[..^1]);
            BuildContextMenu();

            DesktopShortcut = ShortcutFuncs.CheckShortcuts(_appSettings, _currentPlatform.FullName);
            _appData.InitializedClasses.Basic = true;
        }

        public void SaveSettings()
        {
            // Accounts seem to reset when saving, for some reason...
            var accList = _appData.BasicAccounts;
            GeneralFuncs.SaveSettings(_currentPlatform.SettingsFile, this);
            _appData.BasicAccounts = accList;
        }


        [JsonProperty("FolderPath", Order = 1)] private string _folderPath = "";
        public string FolderPath
        {
            get
            {
                if (!string.IsNullOrWhiteSpace(_folderPath)) return _folderPath;
                _folderPath = _currentPlatform.DefaultFolderPath;

                return _folderPath;
            }
            set => _folderPath = value;
        }

        [JsonProperty(Order = 2)] public bool Admin { get; set; }
        [JsonProperty(Order = 3)] public int TrayAccNumber { get; set; } = 3;
        [JsonProperty(Order = 4)] public bool ForgetAccountEnabled { get; set; }
        [JsonProperty(Order = 5)] public Dictionary<int, string> Shortcuts { get; set; } = new();

        [JsonProperty("ClosingMethod", Order = 6)]
        private string _closingMethod = "";

        public string ClosingMethod
        {
            get
            {
                if (!string.IsNullOrWhiteSpace(_closingMethod)) return _closingMethod;
                _closingMethod = _currentPlatform.ClosingMethod;

                return _closingMethod;
            }
            set => _closingMethod = value;
        }

        [JsonProperty("StartingMethod", Order = 7)]
        private string _startingMethod = "";

        public string StartingMethod
        {
            get
            {
                if (!string.IsNullOrWhiteSpace(_startingMethod)) return _startingMethod;
                _startingMethod = _currentPlatform.StartingMethod;

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
                new[]
                {
                    new("Context_SwapTo", new Action(async () => await _appFuncs.SwapToAccount())),
                    new("Context_CopyUsername",
                        new Action(async () => await _appFuncs.CopyText(_appData.SelectedAccount.DisplayName))),
                    new("Context_ChangeName", new Action(_modalData.ShowChangeUsernameModal)),
                    new("Context_CreateShortcut", new Action(async () => await _generalFuncs.CreateShortcut())),
                    new("Context_ChangeImage", new Action(_modalData.ShowChangeAccImageModal)),
                    new("Forget", new Action(async () => await _appFuncs.ForgetAccount())),
                    new("Notes", new Action(() => _modalData.ShowModal("notes"))),
                    _basicStats.PlatformHasAnyGames(_currentPlatform.SafeName)
                        ? new Tuple<string, object>("Context_ManageGameStats", "ShowGameStatsSetup(event)")
                        : null,
                }).Result());
        }

        public ObservableCollection<MenuItem> ContextMenuShortcutItems { get; set; } = new MenuBuilder(
            new Tuple<string, object>[]
            {
                new("Context_RunAdmin", "shortcut('admin')"),
                new("Context_Hide", "shortcut('hide')"),
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
        public string Exe() => Path.Join(FolderPath, _currentPlatform.ExeName);

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

        public void RunPlatform(bool admin)
        {
            _ = Globals.StartProgram(Exe(), admin, _currentPlatform.ExtraArgs, _currentPlatform.StartingMethod)
                ? _generalFuncs.ShowToast("info",
                    _lang["Status_StartingPlatform", new { platform = _currentPlatform.SafeName }], renderTo: "toastarea")
                : _generalFuncs.ShowToast("error",
                    _lang["Toast_StartingPlatformFailed", new { platform = _currentPlatform.SafeName }],
                    renderTo: "toastarea");
        }

        public async Task RunPlatform()
        {
            Globals.StartProgram(Exe(), Admin, _currentPlatform.ExtraArgs, _currentPlatform.StartingMethod);
            await _generalFuncs.ShowToast("info",
                _lang["Status_StartingPlatform", new { platform = _currentPlatform.SafeName }], renderTo: "toastarea");
        }

        public Dictionary<string, string> AccountIds;


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
                    var f = Path.Join(_currentPlatform.ShortcutFolder, shortcut);
                    if (File.Exists(f)) File.Move(f, f.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));

                    // Save.
                    SaveSettings();
                    break;
                }
                case "admin":
                    await BasicFuncs.RunShortcut(_generalFuncs, _currentPlatform, _appStats, shortcut, admin: true);
                    break;
            }
        }

        #region SETTINGS

        /// <summary>
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\ResetSettings]");
            FolderPath = _currentPlatform.DefaultFolderPath;
            Admin = false;
            TrayAccNumber = 3;
            DesktopShortcut = ShortcutFuncs.CheckShortcuts(_appSettings, _currentPlatform.FullName);

            SaveSettings();
        }

        #endregion


        #region Account IDs

        public void LoadAccountIds() => AccountIds = _generalFuncs.ReadDict(_currentPlatform.IdsJsonPath);

        //public static void LoadAccountIds()
        //{
        //    var p = Path.GetFullPath(CurrentPlatform.IdsJsonPath);
        //    AccountIds = GeneralFuncs.ReadDict(p);
        //    return;
        //}
        private void SaveAccountIds() =>
            File.WriteAllText(_currentPlatform.IdsJsonPath, JsonConvert.SerializeObject(AccountIds));

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
            if (_currentPlatform.ExtraArgs != "")
            {
                args = _currentPlatform.ExtraArgs + (args == "" ? "" : " " + args);
            }

            LoadAccountIds();
            var accName = GetNameFromId(accId);

            if (!await KillGameProcesses())
                return;

            // Add currently logged in account if there is a way of checking unique ID.
            // If saved, and has unique key: Update
            if (_currentPlatform.UniqueIdFile is not null)
            {
                var uniqueId = await BasicSettings.GetUniqueId(_currentPlatform, _generalFuncs);

                // UniqueId Found >> Save!
                if (File.Exists(_currentPlatform.IdsJsonPath))
                {
                    if (!string.IsNullOrEmpty(uniqueId) && AccountIds.ContainsKey(uniqueId))
                    {
                        if (accId == uniqueId)
                        {
                            await _generalFuncs.ShowToast("info", _lang["Toast_AlreadyLoggedIn"],
                                renderTo: "toastarea");
                            if (AutoStart)
                            {
                                _ = Globals.StartProgram(Exe(), Admin, args,
                                    _currentPlatform.StartingMethod)
                                    ? _generalFuncs.ShowToast("info",
                                        _lang["Status_StartingPlatform", new {platform = _currentPlatform.SafeName}],
                                        renderTo: "toastarea")
                                    : _generalFuncs.ShowToast("error",
                                        _lang["Toast_StartingPlatformFailed",
                                            new {platform = _currentPlatform.SafeName}], renderTo: "toastarea");
                            }

                            await _appData.InvokeVoidAsync("updateStatus", _lang["Done"]);

                            return;
                        }

                        await BasicAddCurrent(AccountIds[uniqueId]);
                    }
                }
            }

            // Clear current login
            BasicSettings.ClearCurrentLoginBasic(_currentPlatform,_generalFuncs);

            // Copy saved files in
            if (accName != "")
            {
                if (!await BasicCopyInAccount(accId)) return;
                Globals.AddTrayUser(_currentPlatform.SafeName, $"+{_currentPlatform.PrimaryId}:" + accId, accName,
                    TrayAccNumber); // Add to Tray list, using first Identifier
            }

            if (AutoStart) BasicFuncs.RunPlatform(_generalFuncs, Exe(), Admin, args, _currentPlatform.FullName, _currentPlatform.StartingMethod);

            if (accName != "" && AutoStart && _appSettings.MinimizeOnSwitch)
                await _appData.InvokeVoidAsync("hideWindow");

            NativeFuncs.RefreshTrayArea();
            await _appData.InvokeVoidAsync("updateStatus", _lang["Done"]);
            _appStats.IncrementSwitches(_currentPlatform.SafeName);

            try
            {
                LastAccName = accId;
                LastAccTimestamp = Globals.GetUnixTimeInt();
                if (LastAccName != "")
                    await _accountFuncs.SetCurrentAccount(LastAccName);
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
                var uniqueId = await BasicSettings.GetUniqueId(_currentPlatform, _generalFuncs);

                // UniqueId Found in saved file >> return value
                if (File.Exists(_currentPlatform.IdsJsonPath) && !string.IsNullOrEmpty(uniqueId) &&
                    AccountIds.ContainsKey(uniqueId))
                    return uniqueId;
            }
            catch (Exception)
            {
                //
            }

            return "";
        }


        private async Task<bool> BasicCopyInAccount(string accId)
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.BasicCopyInAccount]");
            LoadAccountIds();
            var accName = GetNameFromId(accId);

            var localCachePath = _currentPlatform.AccountLoginCachePath(accName);
            _ = Directory.CreateDirectory(localCachePath);

            if (_currentPlatform.LoginFiles == null)
                throw new Exception("No data in basic platform: " + _currentPlatform.FullName);

            // Get unique ID from IDs file if unique ID is a registry key. Set if exists.
            if (OperatingSystem.IsWindows() && _currentPlatform.UniqueIdMethod is "REGKEY" &&
                !string.IsNullOrEmpty(_currentPlatform.UniqueIdFile))
            {
                var uniqueId = _generalFuncs.ReadDict(_currentPlatform.SafeName).FirstOrDefault(x => x.Value == accName)
                    .Key;

                if (!string.IsNullOrEmpty(uniqueId) &&
                    !Globals.SetRegistryKey(_currentPlatform.UniqueIdFile, uniqueId)) // Remove "REG:" and read data
                {
                    await _generalFuncs.ShowToast("error", _lang["Toast_AlreadyLoggedIn"], _lang["Error"], "toastarea");
                    return false;
                }
            }

            var regJson = _currentPlatform.HasRegistryFiles
                ? _currentPlatform.ReadRegJson(accName)
                : new Dictionary<string, string>();

            foreach (var (accFile, savedFile) in _currentPlatform.LoginFiles)
            {
                // The "file" is a registry key
                if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
                {
                    if (!regJson.ContainsKey(accFile))
                    {
                        await _generalFuncs.ShowToast("error", _lang["Toast_RegFailReadSaved"], _lang["Error"],
                            "toastarea");
                        continue;
                    }

                    var regValue = regJson[accFile] ?? "";

                    if (!Globals.SetRegistryKey(accFile[4..], regValue)) // Remove "REG:" and read data
                    {
                        await _generalFuncs.ShowToast("error", _lang["Toast_RegFailWrite"], _lang["Error"],
                            "toastarea");
                        return false;
                    }

                    continue;
                }

                // The "file" is a JSON value
                if (accFile.StartsWith("JSON"))
                {
                    var jToken = _generalFuncs.ReadJsonFile(Path.Join(localCachePath, savedFile));

                    var path = accFile.Split("::")[1];
                    var selector = accFile.Split("::")[2];
                    if (!Globals.ReplaceVarInJsonFile(path, selector, jToken))
                    {
                        await _generalFuncs.ShowToast("error", _lang["Toast_JsonFailModify"], _lang["Error"],
                            "toastarea");
                        return false;
                    }

                    continue;
                }


                // FILE OR FOLDER
                await BasicSettings.HandleFileOrFolder(_generalFuncs, accFile, savedFile, localCachePath, true);
            }

            return true;
        }

        private async Task<bool> KillGameProcesses()
        {
            // Kill game processes
            await _appData.InvokeVoidAsync("updateStatus",
                _lang["Status_ClosingPlatform", new {platform = _currentPlatform.FullName}]);
            if (await _generalFuncs.CloseProcesses(_currentPlatform.ExesToEnd, ClosingMethod)) return true;

            if (Globals.IsAdministrator)
                await _appData.InvokeVoidAsync("updateStatus",
                    _lang["Status_ClosingPlatformFailed", new {platform = _currentPlatform.FullName}]);
            else
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_RestartAsAdmin"], _lang["Failed"], "toastarea");
                _modalData.ShowModal("confirm", ExtraArg.RestartAsAdmin);
            }

            return false;
        }

        [SupportedOSPlatform("windows")]
        public async Task<bool> BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Basic\Basic.BasicAddCurrent]");
            if (_currentPlatform.ExitBeforeInteract)
                if (!await KillGameProcesses())
                    return false;

            // If set to clear LoginCache for account before adding (Enabled by default):
            if (_currentPlatform.ClearLoginCache)
            {
                Globals.RecursiveDelete(_currentPlatform.AccountLoginCachePath(accName), false);
            }

            // Separate special arguments (if any)
            var specialString = "";
            if (_currentPlatform.HasExtras && accName.Contains(":{"))
            {
                var index = accName.IndexOf(":{", StringComparison.Ordinal)! + 1;
                specialString = accName[index..];
                accName = accName.Split(":{")[0];
            }

            var localCachePath = _currentPlatform.AccountLoginCachePath(accName);
            _ = Directory.CreateDirectory(localCachePath);

            if (_currentPlatform.LoginFiles == null)
                throw new Exception("No data in basic platform: " + _currentPlatform.FullName);

            // Handle unique ID
            await _appData.InvokeVoidAsync("updateStatus", _lang["Status_GetUniqueId"]);

            var uniqueId = await BasicSettings.GetUniqueId(_currentPlatform, _generalFuncs);

            if (uniqueId == "" && _currentPlatform.UniqueIdMethod == "CREATE_ID_FILE")
            {
                // Unique ID file, and does not already exist: Therefore create!
                var uniqueIdFile = _currentPlatform.GetUniqueFilePath();
                uniqueId = Globals.RandomString(16);
                await File.WriteAllTextAsync(uniqueIdFile, uniqueId);
            }

            // Handle special args in username
            var hadSpecialProperties = BasicSettings.ProcessSpecialAccName(_currentPlatform, _generalFuncs, specialString, accName, uniqueId);

            var regJson = _currentPlatform.HasRegistryFiles
                ? _currentPlatform.ReadRegJson(accName)
                : new Dictionary<string, string>();

            await _appData.InvokeVoidAsync("updateStatus", _lang["Status_CopyingFiles"]);
            foreach (var (accFile, savedFile) in _currentPlatform.LoginFiles)
            {
                // HANDLE REGISTRY KEY
                if (accFile.StartsWith("REG:"))
                {
                    var trimmedName = accFile[4..];

                    if (BasicSettings.ReadRegistryKeyWithErrors(_generalFuncs, trimmedName,
                            out var response)) // Remove "REG:                    " and read data
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
                                Globals.WriteToLog(
                                    $"Unexpected registry type encountered (2)! Report to TechNobo. {response.GetType()}");
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
                    var js = await _generalFuncs.ReadJsonFile(path);
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

                    var originalValueString = (string) originalValue;
                    originalValueString = Globals.GetCleanFilePath(firstResult
                        ? originalValueString.Split(delimiter).First()
                        : originalValueString.Split(delimiter).Last());

                    Globals.SaveJsonFile(Path.Join(localCachePath, savedFile), originalValueString);
                    continue;
                }

                // FILE OR FOLDER
                if (await BasicSettings.HandleFileOrFolder(_generalFuncs, accFile, savedFile, localCachePath, false)) continue;

                // Could not find file/folder
                await _generalFuncs.ShowToast("error", _lang["CouldNotFindX", new {x = accFile}],
                    _lang["DirectoryNotFound"], "toastarea");
                return false;

                // TODO: Run some action that can be specified in the Platforms.json file
                // Add for the start, and end of this function -- To allow 'plugins'?
                // Use reflection?
            }

            _currentPlatform.SaveRegJson(regJson, accName);

            var allIds = _generalFuncs.ReadDict(_currentPlatform.IdsJsonPath);
            allIds[uniqueId] = accName;
            await File.WriteAllTextAsync(_currentPlatform.IdsJsonPath, JsonConvert.SerializeObject(allIds));

            // Copy in profile image from default -- As long as not already handled by special arguments
            // Or if has ProfilePicFromFile and ProfilePicRegex.
            if (!hadSpecialProperties.Contains("IMAGE|"))
            {
                await _appData.InvokeVoidAsync("updateStatus", _lang["Status_HandlingImage"]);

                _ = Directory.CreateDirectory(Path.Join(_generalFuncs.WwwRoot(),
                    $"\\img\\profiles\\{_currentPlatform.SafeName}"));
                var profileImg = Path.Join(_generalFuncs.WwwRoot(),
                    $"\\img\\profiles\\{_currentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg");
                if (!File.Exists(profileImg))
                {
                    var platformImgPath = "\\img\\platform\\" + _currentPlatform.SafeName + "Default.png";

                    // Copy in profile picture (if found) from Regex search of files (if defined)
                    if (_currentPlatform.ProfilePicFromFile != "" && _currentPlatform.ProfilePicRegex != "")
                    {
                        var res = Globals.GetCleanFilePath(BasicSettings.RegexSearchFileOrFolder(_currentPlatform.ProfilePicFromFile,
                            _currentPlatform.ProfilePicRegex));
                        var sourcePath = res;
                        if (_currentPlatform.ProfilePicPath != "")
                        {
                            // The regex result should be considered a filename.
                            // Sub in %FileName% from res, and %UniqueId% from uniqueId
                            sourcePath = BasicSettings.ExpandEnvironmentVariables(_currentPlatform.ProfilePicPath
                                .Replace("%FileName%", res).Replace("%UniqueId", uniqueId));
                        }

                        if (res != "" && File.Exists(sourcePath))
                            if (!Globals.CopyFile(sourcePath, profileImg))
                                Globals.WriteToLog(
                                    "Tried to save profile picture from path (ProfilePicFromFile, ProfilePicRegex method)");
                    }
                    else if (_currentPlatform.ProfilePicPath != "")
                    {
                        var sourcePath = BasicSettings.ExpandEnvironmentVariables(
                                Globals.GetCleanFilePath(
                                    _currentPlatform.ProfilePicPath.Replace("%UniqueId", uniqueId))) ?? "";
                        if (sourcePath != "" && File.Exists(sourcePath))
                            if (!Globals.CopyFile(sourcePath, profileImg))
                                Globals.WriteToLog("Tried to save profile picture from path (ProfilePicPath method)");
                    }

                    // Else (If file couldn't be saved, or not found -> Default.
                    if (!File.Exists(profileImg))
                    {
                        var currentPlatformImgPath = Path.Join(_generalFuncs.WwwRoot(), platformImgPath);
                        Globals.CopyFile(File.Exists(currentPlatformImgPath)
                            ? Path.Join(currentPlatformImgPath)
                            : Path.Join(_generalFuncs.WwwRoot(), "\\img\\BasicDefault.png"), profileImg);
                    }
                }
            }

            _appData.NavigateTo(
                $"/Basic/?cacheReload&toast_type=success&toast_title={Uri.EscapeDataString(_lang["Success"])}&toast_message={Uri.EscapeDataString(_lang["Toast_SavedItem", new {item = accName}])}",
                true);
            return true;
        }

        public async Task ChangeUsername(string accId, string newName, bool reload = true)
        {
            LoadAccountIds();
            var oldName = GetNameFromId(accId);

            try
            {
                // No need to rename image as accId. That step is skipped here.
                Directory.Move($"LoginCache\\{_currentPlatform.SafeName}\\{oldName}\\",
                    $"LoginCache\\{_currentPlatform.SafeName}\\{newName}\\"); // Rename login cache folder
            }
            catch (IOException e)
            {
                Globals.WriteToLog("Failed to write to file: " + e);
                await _generalFuncs.ShowToast("error", _lang["Error_FileAccessDenied", new {logPath = Globals.GetLogPath()}], _lang["Error"],
                    renderTo: "toastarea");
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
                await _generalFuncs.ShowToast("error", _lang["Toast_CantChangeUsername"], _lang["Error"],
                    renderTo: "toastarea");
                return;
            }

            if (_appData.SelectedAccount is not null)
            {
                _appData.SelectedAccount.DisplayName = _modalData.TextInput.LastString;
                _appData.SelectedAccount.NotifyDataChanged();
            }

            await _generalFuncs.ShowToast("success", _lang["Toast_ChangedUsername"], renderTo: "toastarea");
        }
    }
}