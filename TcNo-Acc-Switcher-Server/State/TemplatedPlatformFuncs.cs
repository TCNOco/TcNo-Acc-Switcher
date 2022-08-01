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

using Microsoft.AspNetCore.Components;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class TemplatedPlatformFuncs : ITemplatedPlatformFuncs
{
    private readonly IAppState _appState;
    private readonly ILang _lang;
    private readonly IGameStats _gameStats;
    private readonly IModals _modals;
    private readonly ISharedFunctions _sharedFunctions;
    private readonly IStatistics _statistics;
    private readonly ITemplatedPlatformState _templatedPlatformState;
    private readonly ITemplatedPlatformSettings _templatedPlatformSettings;
    private readonly IToasts _toasts;
    private readonly IWindowSettings _windowSettings;

    public TemplatedPlatformFuncs(IAppState appState, ILang lang, IGameStats gameStats, IModals modals, ISharedFunctions sharedFunctions,
        IStatistics statistics, ITemplatedPlatformState templatedPlatformState,
        ITemplatedPlatformSettings templatedPlatformSettings, IToasts toasts, IWindowSettings windowSettings)
    {
        _appState = appState;
        _lang = lang;
        _gameStats = gameStats;
        _modals = modals;
        _sharedFunctions = sharedFunctions;
        _statistics = statistics;
        _templatedPlatformState = templatedPlatformState;
        _templatedPlatformSettings = templatedPlatformSettings;
        _toasts = toasts;
        _windowSettings = windowSettings;
    }

    /// <summary>
    /// Restart Basic with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="jsRuntime"></param>
    /// <param name="accId">(Optional) User's unique account ID</param>
    /// <param name="args">Starting arguments</param>
    [SupportedOSPlatform("windows")]
    public async Task SwapTemplatedAccounts(IJSRuntime jsRuntime, string accId = "", string args = "")
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.SwapTemplatedAccounts] Swapping to: hidden.");
        // Handle args:
        if (_templatedPlatformState.CurrentPlatform.ExeExtraArgs != "")
        {
            args = _templatedPlatformState.CurrentPlatform.ExeExtraArgs + (args == "" ? "" : " " + args);
        }

        _templatedPlatformState.LoadAccountIds();
        var accName = _templatedPlatformState.GetNameFromId(accId);

        if (!KillGameProcesses())
            return;

        // Add currently logged in account if there is a way of checking unique ID.
        // If saved, and has unique key: Update
        if (_templatedPlatformState.CurrentPlatform.UniqueIdFile is not null)
        {
            var uniqueId = GetUniqueId();

            // UniqueId Found >> Save!
            if (File.Exists(_templatedPlatformState.CurrentPlatform.IdsJsonPath)
                && !string.IsNullOrEmpty(uniqueId)
                && _templatedPlatformState.AccountIds.ContainsKey(uniqueId))
            {
                if (accId == uniqueId)
                {
                    await _toasts.ShowToastLangAsync(ToastType.Info, "Toast_AlreadyLoggedIn");
                    if (_templatedPlatformSettings.AutoStart)
                    {
                        if (Globals.StartProgram(_templatedPlatformSettings.Exe,
                                _templatedPlatformSettings.Admin, args,
                                _templatedPlatformState.CurrentPlatform.Extras.StartingMethod))
                            await _toasts.ShowToastLangAsync(ToastType.Info, new LangSub("Status_StartingPlatform", new { platform = _templatedPlatformState.CurrentPlatform.SafeName }));
                        else
                            await _toasts.ShowToastLangAsync(ToastType.Error, new LangSub("Toast_StartingPlatformFailed", new { platform = _templatedPlatformState.CurrentPlatform.SafeName }));
                    }

                    await _appState.Switcher.UpdateStatusAsync(_lang["Done"]);

                    return;
                }

                TemplatedAddCurrent(_templatedPlatformState.AccountIds[uniqueId]);
            }
        }

        // Clear current login
        ClearCurrentLoginBasic();

        // Copy saved files in
        if (accName != "")
        {
            if (!BasicCopyInAccount(accId)) return;
            Globals.AddTrayUser(_templatedPlatformState.CurrentPlatform.SafeName, $"+{_templatedPlatformState.CurrentPlatform.PrimaryId}:" + accId, accName, _templatedPlatformSettings.TrayAccNumber); // Add to Tray list, using first Identifier
        }
        else
            SetCurrentAccount(""); // Else unselect all accounts, as this is a new account.

        if (_templatedPlatformSettings.AutoStart)
            RunPlatform(_templatedPlatformSettings.Admin, args);

        if (accName != "" && _templatedPlatformSettings.AutoStart && _windowSettings.MinimizeOnSwitch) await jsRuntime.InvokeVoidAsync("hideWindow");

        NativeFuncs.RefreshTrayArea();
        await _appState.Switcher.UpdateStatusAsync(_lang["Done"]);
        _statistics.IncrementSwitches(_templatedPlatformState.CurrentPlatform.SafeName);

        try
        {
            _templatedPlatformSettings.LastAccName = accId;
            _templatedPlatformSettings.LastAccTimestamp = Globals.GetUnixTimeInt();
            if (_templatedPlatformSettings.LastAccName != "")
                SetCurrentAccount(_templatedPlatformSettings.LastAccName);
        }
        catch (Exception)
        {
            //
        }
    }

    /// <summary>
    /// Highlights the specified account
    /// </summary>
    public void SetCurrentAccount(string accId)
    {
        var acc = _appState.Switcher.TemplatedAccounts.FirstOrDefault(x => x.AccountId == accId);
        foreach (var account in _appState.Switcher.TemplatedAccounts)
        {
            if (account == acc) continue;
            account.IsCurrent = false;
            account.TitleText = account.Note;
        }

        if (acc is null) return;
        acc.IsCurrent = true;
        acc.TitleText = $"{_lang["Tooltip_CurrentAccount"]}";
    }

    public string GetCurrentAccountId()
    {
        // 30 second window - For when changing accounts
        if (_templatedPlatformSettings.LastAccName != "" && _templatedPlatformSettings.LastAccTimestamp - Globals.GetUnixTimeInt() < 30)
            return _templatedPlatformSettings.LastAccName;

        try
        {
            var uniqueId = GetUniqueId();

            // UniqueId Found in saved file >> return value
            if (File.Exists(_templatedPlatformState.CurrentPlatform.IdsJsonPath) && _templatedPlatformState.AccountIds is not null && !string.IsNullOrEmpty(uniqueId) && _templatedPlatformState.AccountIds.ContainsKey(uniqueId))
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
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ClearCurrentLoginBasic]");

        // Foreach file/folder/reg in Platform.PathListToClear
        if (_templatedPlatformState.CurrentPlatform.PathListToClear.Any(accFile => !DeleteFileOrFolder(accFile))) return;

        if (_templatedPlatformState.CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
        {
            var path = _templatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[0];
            var selector = _templatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[1];
            Globals.ReplaceVarInJsonFile(path, selector, "");
        }

        if (_templatedPlatformState.CurrentPlatform.UniqueIdMethod != "CREATE_ID_FILE") return;

        // Unique ID file --> This needs to be deleted for a new instance
        var uniqueIdFile = _templatedPlatformState.CurrentPlatform.UniqueFilePath;
        Globals.DeleteFile(uniqueIdFile);
    }

    public void ClearCache()
    {

        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ClearCache]");
        var totalFiles = 0;
        var totalSize = Globals.FileLengthToString(_templatedPlatformState.CurrentPlatform.Extras.CachePaths.Sum(x => SizeOfFile(x, ref totalFiles)));
        _toasts.ShowToastLang(ToastType.Info, "Working", new LangSub("Platform_ClearCacheTotal", new { totalFileCount = totalFiles, totalSizeMB = totalSize }));

        // Foreach file/folder/reg in Platform.PathListToClear
        foreach (var f in _templatedPlatformState.CurrentPlatform.Extras.CachePaths.Where(f => !DeleteFileOrFolder(f)))
        {
            _toasts.ShowToastLang(ToastType.Error, "Working", new LangSub("Platform_CouldNotDeleteLog", new { logPath = Globals.GetLogPath() }));
            Globals.WriteToLog("Could not delete: " + f);
        }

        _toasts.ShowToastLang(ToastType.Success, "Working", "DeletedFiles");
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

    private bool DeleteFileOrFolder(string accFile)
    {
        // The "file" is a registry key
        if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
        {
            // If set to clear LoginCache for account before adding (Enabled by default):
            if (_templatedPlatformState.CurrentPlatform.RegDeleteOnClear)
            {
                if (Globals.DeleteRegistryKey(accFile[4..])) return true;
            }
            else
            {
                if (Globals.SetRegistryKey(accFile[4..])) return true;
            }
            _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailWrite");
            return false;
        }

        // The "file" is a JSON value
        if (accFile.StartsWith("JSON"))
        {
            if (accFile.StartsWith("JSON_SELECT"))
            {
                var path = ExpandEnvironmentVariables(accFile.Split("::")[1]);
                var selector = accFile.Split("::")[2];
                Globals.ReplaceVarInJsonFile(path, selector, "");
                return true;
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
                    _toasts.ShowToastLang(ToastType.Error, "Error", "Platform_DeleteFail");
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
                _toasts.ShowToastLang(ToastType.Error, "Error", "Platform_DeleteFail");
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
            _toasts.ShowToastLang(ToastType.Error, "Error", "Platform_DeleteFail");
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
            path = path.Replace("%Platform_Folder%", _templatedPlatformSettings.FolderPath ?? "");

        return Environment.ExpandEnvironmentVariables(path);
    }

    /// <summary>
    /// Copy account files (Switcher -> Platform)
    /// </summary>
    /// <param name="accId"></param>
    /// <returns></returns>
    /// <exception cref="Exception"></exception>
    private bool BasicCopyInAccount(string accId)
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.BasicCopyInAccount]");
        _templatedPlatformState.LoadAccountIds();
        var accName = _templatedPlatformState.GetNameFromId(accId);

        var localCachePath = _templatedPlatformState.CurrentPlatform.AccountLoginCachePath(accName);
        _ = Directory.CreateDirectory(localCachePath);

        if (_templatedPlatformState.CurrentPlatform.LoginFiles == null) throw new Exception("No data in basic platform: " + _templatedPlatformState.CurrentPlatform.Name);

        // Get unique ID from IDs file if unique ID is a registry key. Set if exists.
        if (OperatingSystem.IsWindows() && _templatedPlatformState.CurrentPlatform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(_templatedPlatformState.CurrentPlatform.UniqueIdFile))
        {
            var uniqueId = Globals.ReadDict(_templatedPlatformState.CurrentPlatform.SafeName).FirstOrDefault(x => x.Value == accName).Key;

            if (!string.IsNullOrEmpty(uniqueId) && !Globals.SetRegistryKey(_templatedPlatformState.CurrentPlatform.UniqueIdFile, uniqueId)) // Remove "REG:" and read data
            {
                _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_AlreadyLoggedIn");
                return false;
            }
        }

        var regJson = _templatedPlatformState.CurrentPlatform.HasRegistryFiles ? _templatedPlatformState.CurrentPlatform.ReadRegJson(accName) : new Dictionary<string, string>();

        foreach (var (accFile, savedFile) in _templatedPlatformState.CurrentPlatform.LoginFiles)
        {
            // The "file" is a registry key
            if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
            {
                if (!regJson.ContainsKey(accFile))
                {
                    _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailReadSaved");
                    continue;
                }

                var regValue = regJson[accFile] ?? "";

                if (!Globals.SetRegistryKey(accFile[4..], regValue)) // Remove "REG:" and read data
                {
                    _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailWrite");
                    return false;
                }
                continue;
            }

            // The "file" is a JSON value
            if (accFile.StartsWith("JSON"))
            {
                var jToken = ReadJsonFile(Path.Join(localCachePath, savedFile));

                var path = ExpandEnvironmentVariables(accFile.Split("::")[1]);
                var selector = accFile.Split("::")[2];
                if (!Globals.ReplaceVarInJsonFile(path, selector, jToken))
                {
                    _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_JsonFailModify");
                    return false;
                }
                continue;
            }


            // FILE OR FOLDER
            HandleFileOrFolder(Path.Join(localCachePath, savedFile), accFile);
        }

        return true;
    }

    private bool KillGameProcesses()
    {
        // Kill game processes
        _appState.Switcher.CurrentStatus = _lang["Status_ClosingPlatform", new { platform = _templatedPlatformState.CurrentPlatform.Name }];
        if (_sharedFunctions.CloseProcesses(_templatedPlatformState.CurrentPlatform.ExesToEnd, _templatedPlatformSettings.ClosingMethod)) return true;

        if (Globals.IsAdministrator)
            _appState.Switcher.CurrentStatus = _lang["Status_ClosingPlatformFailed", new { platform = _templatedPlatformState.CurrentPlatform.Name }];
        else
        {
            _toasts.ShowToastLang(ToastType.Error, "Failed", "Toast_RestartAsAdmin");
            _modals.ShowModal("confirm", ExtraArg.RestartAsAdmin);
        }
        return false;

    }

    /// <summary>
    /// Copy account files (Platform -> Switcher)
    /// </summary>
    /// <param name="accName"></param>
    /// <returns></returns>
    /// <exception cref="Exception"></exception>
    public bool TemplatedAddCurrent(string accName)
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.TemplatedAddCurrent]");
        if (_templatedPlatformState.CurrentPlatform.ExitBeforeInteract)
            if (!KillGameProcesses())
                return false;

        // If set to clear LoginCache for account before adding (Enabled by default):
        if (_templatedPlatformState.CurrentPlatform.ClearLoginCache)
        {
            Globals.RecursiveDelete(_templatedPlatformState.CurrentPlatform.AccountLoginCachePath(accName), false);
        }

        // Separate special arguments (if any)
        var specialString = "";
        if (_templatedPlatformState.CurrentPlatform.HasExtras && accName.Contains(":{"))
        {
            var index = accName.IndexOf(":{", StringComparison.Ordinal)! + 1;
            specialString = accName[index..];
            accName = accName.Split(":{")[0];
        }

        var localCachePath = _templatedPlatformState.CurrentPlatform.AccountLoginCachePath(accName);
        _ = Directory.CreateDirectory(localCachePath);

        if (_templatedPlatformState.CurrentPlatform.LoginFiles == null) throw new Exception("No data in basic platform: " + _templatedPlatformState.CurrentPlatform.Name);

        // Handle unique ID
        _appState.Switcher.CurrentStatus = _lang["Status_GetUniqueId"];

        var uniqueId = GetUniqueId();

        if (uniqueId == "" && _templatedPlatformState.CurrentPlatform.UniqueIdMethod == "CREATE_ID_FILE")
        {
            // Unique ID file, and does not already exist: Therefore create!
            var uniqueIdFile = _templatedPlatformState.CurrentPlatform.UniqueFilePath;
            uniqueId = Globals.RandomString(16);
            File.WriteAllText(uniqueIdFile, uniqueId);
        }

        // Handle special args in username
        var hadSpecialProperties = ProcessSpecialAccName(specialString, accName, uniqueId);

        var regJson = _templatedPlatformState.CurrentPlatform.HasRegistryFiles ? _templatedPlatformState.CurrentPlatform.ReadRegJson(accName) : new Dictionary<string, string>();

        _appState.Switcher.CurrentStatus = _lang["Status_CopyingFiles"];
        foreach (var (accFile, savedFile) in _templatedPlatformState.CurrentPlatform.LoginFiles)
        {
            // HANDLE REGISTRY KEY
            if (accFile.StartsWith("REG:"))
            {
                var trimmedName = accFile[4..];

                if (!OperatingSystem.IsWindows()) continue;
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
                var path = ExpandEnvironmentVariables(accFile.Split("::")[1]);
                var selector = accFile.Split("::")[2];
                var js = ReadJsonFile(path);
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
            if (HandleFileOrFolder(accFile, Path.Join(localCachePath, savedFile))) continue;

            // Could not find file/folder
            _toasts.ShowToastLang(ToastType.Error, "DirectoryNotFound", new LangSub("CouldNotFindX", new { x = accFile }));
            return false;

            // TODO: Run some action that can be specified in the Platforms.json file
            // Add for the start, and end of this function -- To allow 'plugins'?
            // Use reflection?
        }

        _templatedPlatformState.CurrentPlatform.SaveRegJson(regJson, accName);

        var allIds = Globals.ReadDict(_templatedPlatformState.CurrentPlatform.IdsJsonPath) ?? new Dictionary<string, string>();
        allIds[uniqueId] = accName;
        File.WriteAllText(_templatedPlatformState.CurrentPlatform.IdsJsonPath, JsonConvert.SerializeObject(allIds));

        // Copy in profile image from default -- As long as not already handled by special arguments
        // Or if has ProfilePicFromFile and ProfilePicRegex.
        var wwwImagePath =
            $"\\img\\profiles\\{_templatedPlatformState.CurrentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg";
        if (!hadSpecialProperties.Contains("IMAGE|"))
        {
            _appState.Switcher.CurrentStatus = _lang["Status_HandlingImage"];

            _ = Directory.CreateDirectory(Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{_templatedPlatformState.CurrentPlatform.SafeName}"));
            var profileImg = Path.Join(Globals.WwwRoot, wwwImagePath);
            if (!File.Exists(profileImg))
            {
                var platformImgPath = "\\img\\platform\\" + _templatedPlatformState.CurrentPlatform.SafeName + "Default.png";
                // Copy in profile picture (if found) from Regex search of files (if defined)
                if (_templatedPlatformState.CurrentPlatform.Extras.ProfilePicFromFile != "" && _templatedPlatformState.CurrentPlatform.Extras.ProfilePicRegex != "")
                {
                    var res = Globals.GetCleanFilePath(RegexSearchFileOrFolder(_templatedPlatformState.CurrentPlatform.Extras.ProfilePicFromFile, _templatedPlatformState.CurrentPlatform.Extras.ProfilePicRegex));
                    var sourcePath = res;
                    if (_templatedPlatformState.CurrentPlatform.Extras.ProfilePicPath != "")
                    {
                        // The regex result should be considered a filename.
                        // Sub in %FileName% from res, and %UniqueId% from uniqueId
                        sourcePath = ExpandEnvironmentVariables(_templatedPlatformState.CurrentPlatform.Extras.ProfilePicPath.Replace("%FileName%", res).Replace("%UniqueId", uniqueId));
                    }

                    if (res != "" && File.Exists(sourcePath))
                        if (!Globals.CopyFile(sourcePath, profileImg))
                            Globals.WriteToLog("Tried to save profile picture from path (ProfilePicFromFile, ProfilePicRegex method)");
                }
                else if (_templatedPlatformState.CurrentPlatform.Extras.ProfilePicPath != "")
                {
                    var sourcePath = ExpandEnvironmentVariables(Globals.GetCleanFilePath(_templatedPlatformState.CurrentPlatform.Extras.ProfilePicPath.Replace("%UniqueId", uniqueId))) ?? "";
                    if (sourcePath != "" && File.Exists(sourcePath))
                        if (!Globals.CopyFile(sourcePath, profileImg))
                            Globals.WriteToLog("Tried to save profile picture from path (ProfilePicPath method)");
                }

                // Else (If file couldn't be saved, or not found -> Default.
                if (!File.Exists(profileImg))
                {
                    var currentPlatformImgPath = Path.Join(Globals.WwwRoot, platformImgPath);
                    Globals.CopyFile(File.Exists(currentPlatformImgPath)
                        ? Path.Join(currentPlatformImgPath)
                        : Path.Join(Globals.WwwRoot, "\\img\\BasicDefault.png"), profileImg);
                }
            }
        }

        // Not in list: Add to dynamically update.
        if (_appState.Switcher.TemplatedAccounts.All(x => x.AccountId != uniqueId))
        {
            var account = new Account
            {
                Platform = _templatedPlatformState.CurrentPlatform.SafeName,
                AccountId = uniqueId,
                DisplayName = accName,
                // Handle account image
                ImagePath = wwwImagePath.Replace("\\", "/"),
                UserStats = _gameStats.GetUserStatsAllGamesMarkup(uniqueId)
            };

            _appState.Switcher.TemplatedAccounts.Add(account);
            SetCurrentAccount(uniqueId);
        }

        _appState.Switcher.CurrentStatus = _lang["Status_AccountSaved"];
        _toasts.ShowToastLang(ToastType.Info, "Status_AccountSaved");
        // NavigationManager.NavigateTo($"/Basic/?cacheReload&toast_type=success&toast_title={Uri.EscapeDataString(_lang["Success"])}&toast_message={Uri.EscapeDataString(_lang["Toast_SavedItem", new { item = accName }])}", true);
        return true;
    }

    /// <summary>
    /// Read a JSON file from provided path. Returns JObject
    /// </summary>
    public JToken ReadJsonFile(string path)
    {
        Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.ReadJsonFile] path={path}");
        JToken jToken = null;

        if (Globals.TryReadJsonFile(path, ref jToken)) return jToken;

        _toasts.ShowToastLang(ToastType.Error, new LangSub("CouldNotReadFile", new { file = path }));
        return new JObject();

    }

    /// <summary>
    /// Handles copying files or folders around
    /// </summary>
    /// <param name="fromPath"></param>
    /// <param name="toPath"></param>
    private bool HandleFileOrFolder(string fromPath, string toPath)
    {
        // Expand, or join localCachePath
        var toFullPath = ExpandEnvironmentVariables(toPath);
        var fromFullPath = ExpandEnvironmentVariables(fromPath);

        // Handle wildcards (Usually filters for copying into the account switcher)
        // From: C:\...\*.log
        // To: LoginCache\<platform>\
        if (fromPath.Contains('*'))
        {
            var folder = Path.GetDirectoryName(fromFullPath) ?? "";
            var file = Path.GetFileName(fromFullPath); // Filename that includes wildcard (*, *.log, f_*)

            // Handle "...\\*" folder.
            if (file == "*")
            {
                if (folder == "" || !Directory.Exists(folder)) return false;
                if (Globals.CopyFilesRecursive(folder, toFullPath)) return true;

                _toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
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

        // Handle wildcards in destination (Usually filters for copying out of account switcher)
        // From: LoginCache\<platform>\
        // To: C:\...\*.log
        if (toPath.Contains('*'))
        {
            var toFolder = Path.GetDirectoryName(toFullPath) ?? "";
            var toFile = Path.GetFileName(toFullPath); // Filename that includes wildcard (*, *.log, f_*)

            // Handle "...\\*" folder.
            Directory.CreateDirectory(toFolder);
            if (toFile == "*")
            {
                if (toFolder == "" || !Directory.Exists(fromFullPath)) return false;
                if (Globals.CopyFilesRecursive(fromFullPath, toFolder)) return true;

                _toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
                return false;
            }

            // Handle "...\\*.log" or "...\\file_*", etc.
            // This is NOT recursive - Specify folders manually in JSON
            foreach (var f in Directory.GetFiles(fromFullPath, toFile))
            {
                if (fromFullPath == null) return false;
                if (fromFullPath.Contains('*')) fromFullPath = Path.GetDirectoryName(fromFullPath);
                var fullOutputPath = Path.Join(fromFullPath, Path.GetFileName(f));
                Globals.CopyFile(f, fullOutputPath);
            }

            return true;
        }

        // Is folder? Recursive copy folder
        if (Directory.Exists(fromFullPath))
        {
            _ = Directory.CreateDirectory(toFullPath);
            if (Globals.CopyFilesRecursive(fromFullPath, toFullPath)) return true;
            _toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
            return false;
        }

        // Is file? Copy file
        if (!File.Exists(fromFullPath)) return false;
        _ = Directory.CreateDirectory(Path.GetDirectoryName(toFullPath) ?? string.Empty);
        var dest = Path.Join(Path.GetDirectoryName(toFullPath), Path.GetFileName(fromFullPath));
        Globals.CopyFile(fromFullPath, dest);
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
        if (!_templatedPlatformState.CurrentPlatform.HasExtras) return hadSpecialProperties;
        var specialProperties = JsonConvert.DeserializeObject<Dictionary<string, string>>(jsonString);
        if (specialProperties == null) return hadSpecialProperties;

        // HANDLE SPECIAL IMAGE
        var profileImg = Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{_templatedPlatformState.CurrentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg");
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

    public string GetUniqueId()
    {
        if (OperatingSystem.IsWindows() && _templatedPlatformState.CurrentPlatform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(_templatedPlatformState.CurrentPlatform.UniqueIdFile))
        {
            if (!ReadRegistryKeyWithErrors(_templatedPlatformState.CurrentPlatform.UniqueIdFile, out var r))
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

        var fileToRead = _templatedPlatformState.CurrentPlatform.UniqueFilePath;
        var uniqueId = "";

        if (_templatedPlatformState.CurrentPlatform.UniqueIdMethod is "CREATE_ID_FILE")
        {
            return File.Exists(fileToRead) ? File.ReadAllText(fileToRead) : uniqueId;
        }

        if (uniqueId == "" && _templatedPlatformState.CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
        {
            JToken js;
            string searchFor;
            if (uniqueId == "" && _templatedPlatformState.CurrentPlatform.UniqueIdMethod == "JSON_SELECT")
            {
                js = ReadJsonFile(_templatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[0]);
                searchFor = _templatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[1];
                uniqueId = Globals.GetCleanFilePath((string)js.SelectToken(searchFor));
                return uniqueId;
            }

            string delimiter;
            var firstResult = true;
            if (_templatedPlatformState.CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT_FIRST"))
            {
                delimiter = _templatedPlatformState.CurrentPlatform.UniqueIdMethod.Split("JSON_SELECT_FIRST")[1];
            }
            else
            {
                delimiter = _templatedPlatformState.CurrentPlatform.UniqueIdMethod.Split("JSON_SELECT_LAST")[1];
                firstResult = false;
            }

            js = ReadJsonFile(_templatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[0]);
            searchFor = _templatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[1];
            var res = (string)js.SelectToken(searchFor);
            if (res is null)
                return "";
            uniqueId = Globals.GetCleanFilePath(firstResult ? res.Split(delimiter).First() : res.Split(delimiter).Last());
            return uniqueId;
        }

        if (fileToRead != null && _templatedPlatformState.CurrentPlatform.UniqueIdFile is not "" && (File.Exists(fileToRead) || fileToRead.Contains('*')))
        {
            if (!string.IsNullOrEmpty(_templatedPlatformState.CurrentPlatform.UniqueIdRegex))
            {
                uniqueId = Globals.GetCleanFilePath(RegexSearchFileOrFolder(fileToRead, _templatedPlatformState.CurrentPlatform.UniqueIdRegex)); // Get unique ID from Regex, but replace any illegal characters.
            }
            else if (_templatedPlatformState.CurrentPlatform.UniqueIdMethod is "FILE_MD5") // TODO: TEST THIS! -- This is used for static files that do not change throughout the lifetime of an account login.
            {
                uniqueId = Globals.GetFileMd5(fileToRead.Contains('*')
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
                _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_AccountIdReg");
                return false;
            case "ERROR-READ":
                _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailRead");
                return false;
        }

        return true;
    }


    public Dictionary<string, string> ReadAllIds(string path = null)
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ReadAllIds]");
        var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
        path ??= _templatedPlatformState.CurrentPlatform.IdsJsonPath;
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

    /// <summary>
    /// Swap to the current AppState.Switcher.SelectedAccountId.
    /// </summary>
    public async Task SwapToAccount(IJSRuntime jsRuntime)
    {
        if (!OperatingSystem.IsWindows()) return;
        await SwapTemplatedAccounts(jsRuntime, _appState.Switcher.SelectedAccountId);
    }

    /// <summary>
    /// Swaps to an empty account, allowing the user to sign in.
    /// </summary>
    public async Task SwapToNewAccount(IJSRuntime jsRuntime)
    {
        if (!OperatingSystem.IsWindows()) return;
        await SwapTemplatedAccounts(jsRuntime, "");
    }

    /// <summary>
    /// Switch to an account, and launch a game via the selected shortcut
    /// Launches the platform, but does not wait for it to start fully.
    /// </summary>
    public async Task SwitchAndLaunchShortcut(IJSRuntime jsRuntime)
    {
        if (_appState.Switcher.SelectedAccount is null)
        {
            await _toasts.ShowToastLangAsync(ToastType.Error, "Toast_SelectAccount");
            return;
        }
        await SwapToAccount(jsRuntime);
        _sharedFunctions.RunShortcut(_appState.Switcher.CurrentShortcut, _templatedPlatformState.CurrentPlatform.ShortcutFolder, _templatedPlatformState.CurrentPlatform.SafeName);
    }

    public void ForgetAccount()
    {
        if (!_templatedPlatformSettings.ForgetAccountEnabled)
            _modals.ShowModal("confirm", ExtraArg.ForgetAccount);
        else
        {
            var trayAcc = _appState.Switcher.SelectedAccountId;
            _templatedPlatformSettings.SetForgetAcc(true);

            // Remove ID from list of ids
            var idsFile = _templatedPlatformState.CurrentPlatform.IdsJsonPath;
            if (File.Exists(idsFile))
            {
                var allIds = Globals.ReadDict(idsFile);
                allIds.Remove(_appState.Switcher.SelectedAccountId);
                File.WriteAllText(idsFile, JsonConvert.SerializeObject(allIds));
            }

            // Remove cached files
            Globals.RecursiveDelete($"LoginCache\\{_appState.Switcher.CurrentSwitcher}\\{_appState.Switcher.SelectedAccount.DisplayName}", false);

            // Remove from Steam accounts list
            _appState.Switcher.TemplatedAccounts.Remove(_appState.Switcher.TemplatedAccounts.First(x => x.AccountId == _appState.Switcher.SelectedAccountId));

            // Remove from Tray
            Globals.RemoveTrayUserByArg(_appState.Switcher.CurrentSwitcher, trayAcc);

            // Remove image
            Globals.DeleteFile(Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{_appState.Switcher.CurrentSwitcher}\\{Globals.GetCleanFilePath(_appState.Switcher.SelectedAccountId)}.jpg"));

            _toasts.ShowToastLang(ToastType.Success, "Success");
        }

        _appState.Switcher.SaveNotes();
        _appState.Switcher.CurrentStatus = _lang["Done"];
    }



    public void RunPlatform(bool admin, string args = "")
    {
        if (Globals.StartProgram(_templatedPlatformSettings.Exe, admin, args, _templatedPlatformState.CurrentPlatform.Extras.StartingMethod))
            _toasts.ShowToastLang(ToastType.Info, new LangSub("Status_StartingPlatform", new { platform = _templatedPlatformState.CurrentPlatform.Name }));
        else
            _toasts.ShowToastLang(ToastType.Error, new LangSub("Toast_StartingPlatformFailed", new { platform = _templatedPlatformState.CurrentPlatform.Name }));
    }

    // This seemed to not be used, so I have omitted it here.
    // public void RunPlatform(bool admin)
    public void RunPlatform() => RunPlatform(_templatedPlatformSettings.Admin, _templatedPlatformState.CurrentPlatform.ExeExtraArgs);
}