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
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class TemplatedPlatformFuncs
{
    [Inject] private Lang Lang { get; set; }
    [Inject] private IAppState AppState { get; set; }
    [Inject] private Toasts Toasts { get; set; }
    [Inject] private IJSRuntime JsRuntime { get; set; }
    [Inject] private Modals Modals { get; set; }
    [Inject] private IStatistics Statistics { get; set; }
    [Inject] private NavigationManager NavigationManager { get; set; }
    [Inject] private IWindowSettings WindowSettings { get; set; }
    [Inject] private TemplatedPlatformState TemplatedPlatformState { get; set; }
    [Inject] private TemplatedPlatformSettings TemplatedPlatformSettings { get; set; }
    [Inject] private SharedFunctions SharedFunctions { get; set; }


    /// <summary>
    /// Restart Basic with a new account selected. Leave args empty to log into a new account.
    /// </summary>
    /// <param name="accId">(Optional) User's unique account ID</param>
    /// <param name="args">Starting arguments</param>
    [SupportedOSPlatform("windows")]
    public async void SwapTemplatedAccounts(string accId = "", string args = "")
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.SwapTemplatedAccounts] Swapping to: hidden.");
        // Handle args:
        if (TemplatedPlatformState.CurrentPlatform.ExeExtraArgs != "")
        {
            args = TemplatedPlatformState.CurrentPlatform.ExeExtraArgs + (args == "" ? "" : " " + args);
        }

        TemplatedPlatformState.LoadAccountIds();
        var accName = TemplatedPlatformState.GetNameFromId(accId);

        if (!await KillGameProcesses())
            return;

        // Add currently logged in account if there is a way of checking unique ID.
        // If saved, and has unique key: Update
        if (TemplatedPlatformState.CurrentPlatform.UniqueIdFile is not null)
        {
            var uniqueId = await GetUniqueId();

            // UniqueId Found >> Save!
            if (File.Exists(TemplatedPlatformState.CurrentPlatform.IdsJsonPath))
            {
                if (!string.IsNullOrEmpty(uniqueId) && TemplatedPlatformState.AccountIds.ContainsKey(uniqueId))
                {
                    if (accId == uniqueId)
                    {
                        Toasts.ShowToastLang(ToastType.Info, "Toast_AlreadyLoggedIn");
                        if (TemplatedPlatformSettings.AutoStart)
                        {
                            if (Globals.StartProgram(TemplatedPlatformSettings.Exe,
                                    TemplatedPlatformSettings.Admin, args,
                                    TemplatedPlatformState.CurrentPlatform.Extras.StartingMethod))
                                Toasts.ShowToastLang(ToastType.Info, new LangSub("Status_StartingPlatform", new { platform = TemplatedPlatformState.CurrentPlatform.SafeName }));
                            else
                                Toasts.ShowToastLang(ToastType.Error, new LangSub("Toast_StartingPlatformFailed", new { platform = TemplatedPlatformState.CurrentPlatform.SafeName }));
                        }
                        await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Done"]);

                        return;
                    }
                    await TemplatedAddCurrent(TemplatedPlatformState.AccountIds[uniqueId]);
                }
            }
        }

        // Clear current login
        ClearCurrentLoginBasic();

        // Copy saved files in
        if (accName != "")
        {
            if (!await BasicCopyInAccount(accId)) return;
            Globals.AddTrayUser(TemplatedPlatformState.CurrentPlatform.SafeName, $"+{TemplatedPlatformState.CurrentPlatform.PrimaryId}:" + accId, accName, TemplatedPlatformSettings.TrayAccNumber); // Add to Tray list, using first Identifier
        }

        if (TemplatedPlatformSettings.AutoStart)
            TemplatedPlatformState.RunPlatform(TemplatedPlatformSettings.Admin, args);

        if (accName != "" && TemplatedPlatformSettings.AutoStart && WindowSettings.MinimizeOnSwitch) await JsRuntime.InvokeVoidAsync("hideWindow");

        NativeFuncs.RefreshTrayArea();
        await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Done"]);
        Statistics.IncrementSwitches(TemplatedPlatformState.CurrentPlatform.SafeName);

        try
        {
            TemplatedPlatformSettings.LastAccName = accId;
            TemplatedPlatformSettings.LastAccTimestamp = Globals.GetUnixTimeInt();
            if (TemplatedPlatformSettings.LastAccName != "")
                await SetCurrentAccount(TemplatedPlatformSettings.LastAccName);
        }
        catch (Exception)
        {
            //
        }
    }

    /// <summary>
    /// Highlights the specified account
    /// </summary>
    public async Task SetCurrentAccount(string accId)
    {
        var acc = AppState.Switcher.TemplatedAccounts.First(x => x.AccountId == accId);
        await UnCurrentAllAccounts();
        acc.IsCurrent = true;
        acc.TitleText = $"{Lang["Tooltip_CurrentAccount"]}";

        // getBestOffset
        await JsRuntime.InvokeVoidAsync("setBestOffset", acc.AccountId);
        // then initTooltips
        await JsRuntime.InvokeVoidAsync("initTooltips");
    }


    /// <summary>
    /// Removes "currently logged in" border from all accounts
    /// </summary>
    public async Task UnCurrentAllAccounts()
    {
        foreach (var account in AppState.Switcher.TemplatedAccounts)
        {
            account.IsCurrent = false;
        }

        // Clear the hover text
        await JsRuntime.InvokeVoidAsync("clearAccountTooltips");
    }

    public async Task<string> GetCurrentAccountId()
    {
        // 30 second window - For when changing accounts
        if (TemplatedPlatformSettings.LastAccName != "" && TemplatedPlatformSettings.LastAccTimestamp - Globals.GetUnixTimeInt() < 30)
            return TemplatedPlatformSettings.LastAccName;

        try
        {
            var uniqueId = await GetUniqueId();

            // UniqueId Found in saved file >> return value
            if (File.Exists(TemplatedPlatformState.CurrentPlatform.IdsJsonPath) && !string.IsNullOrEmpty(uniqueId) && TemplatedPlatformState.AccountIds.ContainsKey(uniqueId))
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
        if (TemplatedPlatformState.CurrentPlatform.PathListToClear.Any(accFile => !DeleteFileOrFolder(accFile).Result)) return;

        if (TemplatedPlatformState.CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
        {
            var path = TemplatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[0];
            var selector = TemplatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[1];
            Globals.ReplaceVarInJsonFile(path, selector, "");
        }

        if (TemplatedPlatformState.CurrentPlatform.UniqueIdMethod != "CREATE_ID_FILE") return;

        // Unique ID file --> This needs to be deleted for a new instance
        var uniqueIdFile = TemplatedPlatformState.CurrentPlatform.UniqueFilePath;
        Globals.DeleteFile(uniqueIdFile);
    }

    public async Task ClearCache()
    {

        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ClearCache]");
        var totalFiles = 0;
        var totalSize = Globals.FileLengthToString(TemplatedPlatformState.CurrentPlatform.Extras.CachePaths.Sum(x => SizeOfFile(x, ref totalFiles)));
        Toasts.ShowToastLang(ToastType.Info, "Working", new LangSub("Platform_ClearCacheTotal", new { totalFileCount = totalFiles, totalSizeMB = totalSize }));

        // Foreach file/folder/reg in Platform.PathListToClear
        foreach (var f in TemplatedPlatformState.CurrentPlatform.Extras.CachePaths.Where(f => !DeleteFileOrFolder(f).Result))
        {
            Toasts.ShowToastLang(ToastType.Error, "Working", new LangSub("Platform_CouldNotDeleteLog", new { logPath = Globals.GetLogPath() }));
            Globals.WriteToLog("Could not delete: " + f);
        }

        Toasts.ShowToastLang(ToastType.Success, "Working", "DeletedFiles");
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
            if (TemplatedPlatformState.CurrentPlatform.RegDeleteOnClear)
            {
                if (Globals.DeleteRegistryKey(accFile[4..])) return true;
            }
            else
            {
                if (Globals.SetRegistryKey(accFile[4..])) return true;
            }
            Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailWrite");
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
                    Toasts.ShowToastLang(ToastType.Error, "Error", "Platform_DeleteFail");
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
                Toasts.ShowToastLang(ToastType.Error, "Error", "Platform_DeleteFail");
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
            Toasts.ShowToastLang(ToastType.Error, "Error", "Platform_DeleteFail");
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
            path = path.Replace("%Platform_Folder%", TemplatedPlatformSettings.FolderPath ?? "");

        return Environment.ExpandEnvironmentVariables(path);
    }

    private async Task<bool> BasicCopyInAccount(string accId)
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.BasicCopyInAccount]");
        TemplatedPlatformState.LoadAccountIds();
        var accName = TemplatedPlatformState.GetNameFromId(accId);

        var localCachePath = TemplatedPlatformState.CurrentPlatform.AccountLoginCachePath(accName);
        _ = Directory.CreateDirectory(localCachePath);

        if (TemplatedPlatformState.CurrentPlatform.LoginFiles == null) throw new Exception("No data in basic platform: " + TemplatedPlatformState.CurrentPlatform.Name);

        // Get unique ID from IDs file if unique ID is a registry key. Set if exists.
        if (OperatingSystem.IsWindows() && TemplatedPlatformState.CurrentPlatform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(TemplatedPlatformState.CurrentPlatform.UniqueIdFile))
        {
            var uniqueId = Globals.ReadDict(TemplatedPlatformState.CurrentPlatform.SafeName).FirstOrDefault(x => x.Value == accName).Key;

            if (!string.IsNullOrEmpty(uniqueId) && !Globals.SetRegistryKey(TemplatedPlatformState.CurrentPlatform.UniqueIdFile, uniqueId)) // Remove "REG:" and read data
            {
                Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_AlreadyLoggedIn");
                return false;
            }
        }

        var regJson = TemplatedPlatformState.CurrentPlatform.HasRegistryFiles ? TemplatedPlatformState.CurrentPlatform.ReadRegJson(accName) : new Dictionary<string, string>();

        foreach (var (accFile, savedFile) in TemplatedPlatformState.CurrentPlatform.LoginFiles)
        {
            // The "file" is a registry key
            if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
            {
                if (!regJson.ContainsKey(accFile))
                {
                    Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailReadSaved");
                    continue;
                }

                var regValue = regJson[accFile] ?? "";

                if (!Globals.SetRegistryKey(accFile[4..], regValue)) // Remove "REG:" and read data
                {
                    Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailWrite");
                    return false;
                }
                continue;
            }

            // The "file" is a JSON value
            if (accFile.StartsWith("JSON"))
            {
                var jToken = ReadJsonFile(Path.Join(localCachePath, savedFile));

                var path = accFile.Split("::")[1];
                var selector = accFile.Split("::")[2];
                if (!Globals.ReplaceVarInJsonFile(path, selector, jToken))
                {
                    Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_JsonFailModify");
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
        await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatform", new { platform = TemplatedPlatformState.CurrentPlatform.Name }]);
        if (await SharedFunctions.CloseProcesses(TemplatedPlatformState.CurrentPlatform.ExesToEnd, TemplatedPlatformSettings.ClosingMethod)) return true;

        if (Globals.IsAdministrator)
            await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = TemplatedPlatformState.CurrentPlatform.Name }]);
        else
        {
            Toasts.ShowToastLang(ToastType.Error, "Failed", "Toast_RestartAsAdmin");
            Modals.ShowModal("confirm", ExtraArg.RestartAsAdmin);
        }
        return false;

    }

    public async Task<bool> TemplatedAddCurrent(string accName)
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.TemplatedAddCurrent]");
        if (TemplatedPlatformState.CurrentPlatform.ExitBeforeInteract)
            if (!await KillGameProcesses())
                return false;

        // If set to clear LoginCache for account before adding (Enabled by default):
        if (TemplatedPlatformState.CurrentPlatform.ClearLoginCache)
        {
            Globals.RecursiveDelete(TemplatedPlatformState.CurrentPlatform.AccountLoginCachePath(accName), false);
        }

        // Separate special arguments (if any)
        var specialString = "";
        if (TemplatedPlatformState.CurrentPlatform.HasExtras && accName.Contains(":{"))
        {
            var index = accName.IndexOf(":{", StringComparison.Ordinal)! + 1;
            specialString = accName[index..];
            accName = accName.Split(":{")[0];
        }

        var localCachePath = TemplatedPlatformState.CurrentPlatform.AccountLoginCachePath(accName);
        _ = Directory.CreateDirectory(localCachePath);

        if (TemplatedPlatformState.CurrentPlatform.LoginFiles == null) throw new Exception("No data in basic platform: " + TemplatedPlatformState.CurrentPlatform.Name);

        // Handle unique ID
        await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Status_GetUniqueId"]);

        var uniqueId = await GetUniqueId();

        if (uniqueId == "" && TemplatedPlatformState.CurrentPlatform.UniqueIdMethod == "CREATE_ID_FILE")
        {
            // Unique ID file, and does not already exist: Therefore create!
            var uniqueIdFile = TemplatedPlatformState.CurrentPlatform.UniqueFilePath;
            uniqueId = Globals.RandomString(16);
            await File.WriteAllTextAsync(uniqueIdFile, uniqueId);
        }

        // Handle special args in username
        var hadSpecialProperties = ProcessSpecialAccName(specialString, accName, uniqueId);

        var regJson = TemplatedPlatformState.CurrentPlatform.HasRegistryFiles ? TemplatedPlatformState.CurrentPlatform.ReadRegJson(accName) : new Dictionary<string, string>();

        await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Status_CopyingFiles"]);
        foreach (var (accFile, savedFile) in TemplatedPlatformState.CurrentPlatform.LoginFiles)
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
                var path = accFile.Split("::")[1];
                var selector = accFile.Split("::")[2];
                var js = await ReadJsonFile(path);
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
            Toasts.ShowToastLang(ToastType.Error, "DirectoryNotFound", new LangSub("CouldNotFindX", new { x = accFile }));
            return false;

            // TODO: Run some action that can be specified in the Platforms.json file
            // Add for the start, and end of this function -- To allow 'plugins'?
            // Use reflection?
        }

        TemplatedPlatformState.CurrentPlatform.SaveRegJson(regJson, accName);

        var allIds = Globals.ReadDict(TemplatedPlatformState.CurrentPlatform.IdsJsonPath);
        allIds[uniqueId] = accName;
        await File.WriteAllTextAsync(TemplatedPlatformState.CurrentPlatform.IdsJsonPath, JsonConvert.SerializeObject(allIds));

        // Copy in profile image from default -- As long as not already handled by special arguments
        // Or if has ProfilePicFromFile and ProfilePicRegex.
        if (!hadSpecialProperties.Contains("IMAGE|"))
        {
            await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Status_HandlingImage"]);

            _ = Directory.CreateDirectory(Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{TemplatedPlatformState.CurrentPlatform.SafeName}"));
            var profileImg = Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{TemplatedPlatformState.CurrentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg");
            if (!File.Exists(profileImg))
            {
                var platformImgPath = "\\img\\platform\\" + TemplatedPlatformState.CurrentPlatform.SafeName + "Default.png";

                // Copy in profile picture (if found) from Regex search of files (if defined)
                if (TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicFromFile != "" && TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicRegex != "")
                {
                    var res = Globals.GetCleanFilePath(RegexSearchFileOrFolder(TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicFromFile, TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicRegex));
                    var sourcePath = res;
                    if (TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicPath != "")
                    {
                        // The regex result should be considered a filename.
                        // Sub in %FileName% from res, and %UniqueId% from uniqueId
                        sourcePath = ExpandEnvironmentVariables(TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicPath.Replace("%FileName%", res).Replace("%UniqueId", uniqueId));
                    }

                    if (res != "" && File.Exists(sourcePath))
                        if (!Globals.CopyFile(sourcePath, profileImg))
                            Globals.WriteToLog("Tried to save profile picture from path (ProfilePicFromFile, ProfilePicRegex method)");
                }
                else if (TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicPath != "")
                {
                    var sourcePath = ExpandEnvironmentVariables(Globals.GetCleanFilePath(TemplatedPlatformState.CurrentPlatform.Extras.ProfilePicPath.Replace("%UniqueId", uniqueId))) ?? "";
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

        NavigationManager.NavigateTo(
            $"/Basic/?cacheReload&toast_type=success&toast_title={Uri.EscapeDataString(Lang["Success"])}&toast_message={Uri.EscapeDataString(Lang["Toast_SavedItem", new { item = accName }])}", true);
        return true;
    }

    /// <summary>
    /// Read a JSON file from provided path. Returns JObject
    /// </summary>
    public async Task<JToken> ReadJsonFile(string path)
    {
        Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.ReadJsonFile] path={path}");
        JToken jToken = null;

        if (Globals.TryReadJsonFile(path, ref jToken)) return jToken;

        Toasts.ShowToastLang(ToastType.Error, new LangSub("CouldNotReadFile", new { file = path }));
        return new JObject();

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

                Toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
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
            Toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
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
        if (!TemplatedPlatformState.CurrentPlatform.HasExtras) return hadSpecialProperties;
        var specialProperties = JsonConvert.DeserializeObject<Dictionary<string, string>>(jsonString);
        if (specialProperties == null) return hadSpecialProperties;

        // HANDLE SPECIAL IMAGE
        var profileImg = Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{TemplatedPlatformState.CurrentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg");
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
        if (OperatingSystem.IsWindows() && TemplatedPlatformState.CurrentPlatform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(TemplatedPlatformState.CurrentPlatform.UniqueIdFile))
        {
            if (!ReadRegistryKeyWithErrors(TemplatedPlatformState.CurrentPlatform.UniqueIdFile, out var r))
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

        var fileToRead = TemplatedPlatformState.CurrentPlatform.UniqueFilePath;
        var uniqueId = "";

        if (TemplatedPlatformState.CurrentPlatform.UniqueIdMethod is "CREATE_ID_FILE")
        {
            return File.Exists(fileToRead) ? await File.ReadAllTextAsync(fileToRead) : uniqueId;
        }

        if (uniqueId == "" && TemplatedPlatformState.CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
        {
            JToken js;
            string searchFor;
            if (uniqueId == "" && TemplatedPlatformState.CurrentPlatform.UniqueIdMethod == "JSON_SELECT")
            {
                js = await ReadJsonFile(TemplatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[0]);
                searchFor = TemplatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[1];
                uniqueId = Globals.GetCleanFilePath((string)js.SelectToken(searchFor));
                return uniqueId;
            }

            string delimiter;
            var firstResult = true;
            if (TemplatedPlatformState.CurrentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT_FIRST"))
            {
                delimiter = TemplatedPlatformState.CurrentPlatform.UniqueIdMethod.Split("JSON_SELECT_FIRST")[1];
            }
            else
            {
                delimiter = TemplatedPlatformState.CurrentPlatform.UniqueIdMethod.Split("JSON_SELECT_LAST")[1];
                firstResult = false;
            }

            js = await ReadJsonFile(TemplatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[0]);
            searchFor = TemplatedPlatformState.CurrentPlatform.UniqueFilePath.Split("::")[1];
            var res = (string)js.SelectToken(searchFor);
            if (res is null)
                return "";
            uniqueId = Globals.GetCleanFilePath(firstResult ? res.Split(delimiter).First() : res.Split(delimiter).Last());
            return uniqueId;
        }

        if (fileToRead != null && TemplatedPlatformState.CurrentPlatform.UniqueIdFile is not "" && (File.Exists(fileToRead) || fileToRead.Contains('*')))
        {
            if (!string.IsNullOrEmpty(TemplatedPlatformState.CurrentPlatform.UniqueIdRegex))
            {
                uniqueId = Globals.GetCleanFilePath(RegexSearchFileOrFolder(fileToRead, TemplatedPlatformState.CurrentPlatform.UniqueIdRegex)); // Get unique ID from Regex, but replace any illegal characters.
            }
            else if (TemplatedPlatformState.CurrentPlatform.UniqueIdMethod is "FILE_MD5") // TODO: TEST THIS! -- This is used for static files that do not change throughout the lifetime of an account login.
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
                Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_AccountIdReg");
                return false;
            case "ERROR-READ":
                Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_RegFailRead");
                return false;
        }

        return true;
    }


    public Dictionary<string, string> ReadAllIds(string path = null)
    {
        Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ReadAllIds]");
        var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
        path ??= TemplatedPlatformState.CurrentPlatform.IdsJsonPath;
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
    public void SwapToAccount()
    {
        SwapTemplatedAccounts(AppState.Switcher.SelectedAccountId);
    }

    /// <summary>
    /// Swaps to an empty account, allowing the user to sign in.
    /// </summary>
    public void SwapToNewAccount()
    {
        if (!OperatingSystem.IsWindows()) return;
        SwapTemplatedAccounts("");
    }

    public async Task ForgetAccount()
    {
        if (!TemplatedPlatformSettings.ForgetAccountEnabled)
            Modals.ShowModal("confirm", ExtraArg.ForgetAccount);
        else
        {
            var trayAcc = AppState.Switcher.SelectedAccountId;
            TemplatedPlatformSettings.SetForgetAcc(true);

            // Remove ID from list of ids
            var idsFile = $"LoginCache\\{AppState.Switcher.CurrentSwitcher}\\ids.json";
            if (File.Exists(idsFile))
            {
                var allIds = Globals.ReadDict(idsFile).Remove(AppState.Switcher.SelectedAccountId);
                await File.WriteAllTextAsync(idsFile, JsonConvert.SerializeObject(allIds));
            }

            // Remove cached files
            Globals.RecursiveDelete($"LoginCache\\{AppState.Switcher.CurrentSwitcher}\\{AppState.Switcher.SelectedAccountId}", false);

            // Remove from Steam accounts list
            AppState.Switcher.TemplatedAccounts.Remove(AppState.Switcher.TemplatedAccounts.First(x => x.AccountId == AppState.Switcher.SelectedAccountId));

            // Remove from Tray
            Globals.RemoveTrayUserByArg(AppState.Switcher.CurrentSwitcher, trayAcc);

            // Remove image
            Globals.DeleteFile(Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{AppState.Switcher.CurrentSwitcher}\\{Globals.GetCleanFilePath(AppState.Switcher.SelectedAccountId)}.jpg"));

            Toasts.ShowToastLang(ToastType.Success, "Success");
        }
    }
}