using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Data.Settings;

internal static class BasicSettings
{
    /// <summary>
    /// Main function for Basic Account Switcher. Run on load.
    /// Collects accounts from cache folder
    /// Prepares HTML Elements string for insertion into the account switcher GUI.
    /// </summary>
    /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
    public static async Task LoadProfiles(IGeneralFuncs generalFuncs, ICurrentPlatform currentPlatform)
    {
        Globals.DebugWriteLine(@"[Func:Basic\Basic.LoadProfiles] Loading Basic profiles for: " + currentPlatform.FullName);
        await generalFuncs.GenericLoadAccounts(currentPlatform.FullName, true);
    }

    [SupportedOSPlatform("windows")]
    public static void ClearCurrentLoginBasic(ICurrentPlatform currentPlatform, IGeneralFuncs generalFuncs)
    {
        Globals.DebugWriteLine(@"[Func:Basic\Basic.ClearCurrentLoginBasic]");

        // Foreach file/folder/reg in Platform.PathListToClear
        if (currentPlatform.PathListToClear.Any(accFile => !DeleteFileOrFolder(generalFuncs, currentPlatform, accFile).Result)) return;

        if (currentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
        {
            var path = currentPlatform.GetUniqueFilePath().Split("::")[0];
            var selector = currentPlatform.GetUniqueFilePath().Split("::")[1];
            Globals.ReplaceVarInJsonFile(path, selector, "");
        }

        if (currentPlatform.UniqueIdMethod != "CREATE_ID_FILE") return;

        // Unique ID file --> This needs to be deleted for a new instance
        var uniqueIdFile = currentPlatform.GetUniqueFilePath();
        Globals.DeleteFile(uniqueIdFile);
    }

    public static async Task ClearCache(ICurrentPlatform currentPlatform, IGeneralFuncs generalFuncs)
    {
        Globals.DebugWriteLine(@"[Func:Basic\Basic.ClearCache]");
        var totalFiles = 0;
        var totalSize =
            Globals.FileLengthToString(currentPlatform.CachePaths.Sum(x => SizeOfFile(x, ref totalFiles)));
        await generalFuncs.ShowToastLangVars("info",new LangItem("Platform_ClearCacheTotal", new { totalFileCount = totalFiles, totalSizeMB = totalSize }), "Working", "toastarea");

        // Foreach file/folder/reg in Platform.PathListToClear
        foreach (var f in currentPlatform.CachePaths.Where(f => !DeleteFileOrFolder(generalFuncs, currentPlatform, f).Result))
        {
            await generalFuncs.ShowToastLangVars("error", new LangItem("Platform_CouldNotDeleteLog", new { logPath = Globals.GetLogPath() }), "Working", "toastarea");
            Globals.WriteToLog("Could not delete: " + f);
        }

        await generalFuncs.ShowToastLangVars("success", "DeletedFiles", "Working", "toastarea");
    }

    private static long SizeOfFile(string accFile, ref int numFiles)
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

    private static async Task<bool> DeleteFileOrFolder(IGeneralFuncs generalFuncs, ICurrentPlatform currentPlatform, string accFile)
    {
        // The "file" is a registry key
        if (OperatingSystem.IsWindows() && accFile.StartsWith("REG:"))
        {
            // If set to clear LoginCache for account before adding (Enabled by default):
            if (currentPlatform.RegDeleteOnClear)
            {
                if (Globals.DeleteRegistryKey(accFile[4..])) return true;
            }
            else
            {
                if (Globals.SetRegistryKey(accFile[4..])) return true;
            }

            await generalFuncs.ShowToastLangVars("error", "Toast_RegFailWrite", "Error", "toastarea");
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
                if (!Directory.Exists(Path.GetDirectoryName((string) folder)))
                    return true;
                if (!Globals.RecursiveDelete(folder, false))
                    await generalFuncs.ShowToastLangVars("error", "Platform_DeleteFail", "Error", "toastarea");
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
                await generalFuncs.ShowToastLangVars("error", "Platform_DeleteFail", "Error", "toastarea");
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
            await generalFuncs.ShowToastLangVars("error", "Platform_DeleteFail", "Error", "toastarea");
        }

        return true;
    }

    /// <summary>
    /// Get string contents of registry file, or path to file matching regex/wildcards.
    /// </summary>
    /// <param name="accFile"></param>
    /// <param name="regex"></param>
    /// <returns></returns>
    public static string RegexSearchFileOrFolder(string accFile, string regex)
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
                    Globals.WriteToLog(
                        $"REG was read, and was returned something that is not a string or byte array! {accFile}.");
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
    /// <param name="platformFolder">If set, will replace %Platform_Folder% as well</param>
    /// <returns></returns>
    public static string ExpandEnvironmentVariables(string path, string platformFolder = "")
    {
        path = Globals.ExpandEnvironmentVariables(path);
        path = path.Replace("%Platform_Folder%", platformFolder);
        return Environment.ExpandEnvironmentVariables(path);
    }

    /// <summary>
    /// Handles copying files or folders around
    /// </summary>
    /// <param name="fromPath"></param>
    /// <param name="toPath"></param>
    /// <param name="localCachePath"></param>
    /// <param name="reverse">FALSE: Platform -> LoginCache. TRUE: LoginCache -> J••Platform</param>
    public static async Task<bool> HandleFileOrFolder(IGeneralFuncs generalFuncs, string fromPath, string toPath, string localCachePath, bool reverse)
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

                await generalFuncs.ShowToastLangVars("error", "Toast_FileCopyFail", renderTo: "toastarea");
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
            await generalFuncs.ShowToastLangVars("error", "Toast_FileCopyFail", renderTo: "toastarea");
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
    public static string ProcessSpecialAccName(ICurrentPlatform currentPlatform, IGeneralFuncs generalFuncs, string jsonString, string accName, string uniqueId)
    {
        // Verify existence of possible extra properties
        var hadSpecialProperties = "";
        if (!currentPlatform.HasExtras) return hadSpecialProperties;
        var specialProperties = JsonConvert.DeserializeObject<Dictionary<string, string>>(jsonString);
        if (specialProperties == null) return hadSpecialProperties;

        // HANDLE SPECIAL IMAGE
        var profileImg = Path.Join(generalFuncs.WwwRoot(),
            $"\\img\\profiles\\{currentPlatform.SafeName}\\{Globals.GetCleanFilePath(uniqueId)}.jpg");
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

    public static async Task<string> GetUniqueId(ICurrentPlatform currentPlatform, IGeneralFuncs generalFuncs)
    {
        if (OperatingSystem.IsWindows() && currentPlatform.UniqueIdMethod is "REGKEY" &&
            !string.IsNullOrEmpty(currentPlatform.UniqueIdFile))
        {
            if (!ReadRegistryKeyWithErrors(generalFuncs, currentPlatform.UniqueIdFile, out var r))
                return "";

            switch (r)
            {
                case string s:
                    return s;
                case byte[]:
                    return Globals.GetSha256HashString(r);
                default:
                    Globals.WriteToLog(
                        $"Unexpected registry type encountered (1)! Report to TechNobo. {r.GetType()}");
                    return "";
            }
        }

        var fileToRead = currentPlatform.GetUniqueFilePath();
        var uniqueId = "";

        if (currentPlatform.UniqueIdMethod is "CREATE_ID_FILE")
            return File.Exists(fileToRead) ? await File.ReadAllTextAsync(fileToRead) : uniqueId;

        if (uniqueId == "" && currentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT"))
        {
            JToken js;
            string searchFor;
            if (uniqueId == "" && currentPlatform.UniqueIdMethod == "JSON_SELECT")
            {
                js = await generalFuncs.ReadJsonFile(currentPlatform.GetUniqueFilePath().Split("::")[0]);
                searchFor = currentPlatform.GetUniqueFilePath().Split("::")[1];
                uniqueId = Globals.GetCleanFilePath((string) js.SelectToken(searchFor));
                return uniqueId;
            }

            string delimiter;
            var firstResult = true;
            if (currentPlatform.UniqueIdMethod.StartsWith("JSON_SELECT_FIRST"))
            {
                delimiter = currentPlatform.UniqueIdMethod.Split("JSON_SELECT_FIRST")[1];
            }
            else
            {
                delimiter = currentPlatform.UniqueIdMethod.Split("JSON_SELECT_LAST")[1];
                firstResult = false;
            }

            js = await generalFuncs.ReadJsonFile(currentPlatform.GetUniqueFilePath().Split("::")[0]);
            searchFor = currentPlatform.GetUniqueFilePath().Split("::")[1];
            var res = (string) js.SelectToken(searchFor);
            if (res is null)
                return "";
            uniqueId = Globals.GetCleanFilePath(firstResult
                ? res.Split(delimiter).First()
                : res.Split(delimiter).Last());
            return uniqueId;
        }

        if (fileToRead != null && currentPlatform.UniqueIdFile is not "" &&
            (File.Exists(fileToRead) || fileToRead.Contains('*')))
        {
            if (!string.IsNullOrEmpty(currentPlatform.UniqueIdRegex))
            {
                uniqueId = Globals.GetCleanFilePath(RegexSearchFileOrFolder(fileToRead,
                    currentPlatform
                        .UniqueIdRegex)); // Get unique ID from Regex, but replace any illegal characters.
            }
            else if
                (currentPlatform
                     .UniqueIdMethod is
                 "FILE_MD5") // TODO: TEST THIS! -- This is used for static files that do not change throughout the lifetime of an account login.
            {
                uniqueId = generalFuncs.GetFileMd5(fileToRead.Contains('*')
                    ? Directory.GetFiles(Path.GetDirectoryName(fileToRead) ?? string.Empty,
                        Path.GetFileName(fileToRead)).First()
                    : fileToRead);
            }
        }
        else if (uniqueId != "")
            uniqueId = Globals.GetSha256HashString(uniqueId);

        return uniqueId;
    }

    [SupportedOSPlatform("windows")]
    public static bool ReadRegistryKeyWithErrors(IGeneralFuncs generalFuncs, string key, out dynamic value)
    {
        value = Globals.ReadRegistryKey(key);
        switch (value)
        {
            case "ERROR-NULL":
                _ = generalFuncs.ShowToastLangVars("error", "Toast_AccountIdReg", "Error", "toastarea");
                return false;
            case "ERROR-READ":
                _ = generalFuncs.ShowToastLangVars("error", "Toast_RegFailRead", "Error", "toastarea");
                return false;
        }

        return true;
    }

    public static Dictionary<string, string> ReadAllIds(ICurrentPlatform currentPlatform, string path = null)
    {
        Globals.DebugWriteLine(@"[Func:Basic\Basic.ReadAllIds]");
        var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
        path ??= currentPlatform.IdsJsonPath;
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