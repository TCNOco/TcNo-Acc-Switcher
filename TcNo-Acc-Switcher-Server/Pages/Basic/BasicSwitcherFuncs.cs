using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.Versioning;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Basic
{
    public class BasicSwitcherFuncs
    {
        private static readonly Lang Lang = Lang.Instance;

        private static readonly Data.Settings.Basic Basic = Data.Settings.Basic.Instance;
        private static readonly CurrentPlatform Platform = CurrentPlatform.Instance;
        /// <summary>
        /// Main function for Basic Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static void LoadProfiles()
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.LoadProfiles] Loading Basic profiles for: " + Platform.FullName);
            Data.Settings.Basic.Instance.LoadFromFile();
            _ = GenericFunctions.GenericLoadAccounts(Platform.FullName);
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetBasicForgetAcc() => Task.FromResult(Basic.ForgetAccountEnabled);

        /// <summary>
        /// Restart Basic with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        /// <param name="args">Starting arguments</param>
        [SupportedOSPlatform("windows")]
        public static void SwapBasicAccounts(string accName = "", string args = "")
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.SwapBasicAccounts] Swapping to: hidden.");

            // Kill game processes
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatform", new { platform = "Basic" }]);
            if (!GeneralFuncs.CloseProcesses(Platform.ExesToEnd, Data.Settings.Basic.Instance.AltClose))
            {
                _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = Platform.FullName }]);
                return;
            };

            // If saved, and has unique key: Update
            if (Platform.UniqueIdFile is not null)
            {
                string uniqueId;
                if (Platform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(Platform.UniqueIdFile))
                {
                    _ = ReadRegistryKeyWithErrors(Platform.UniqueIdFile, out uniqueId);
                }
                else
                    uniqueId = GetUniqueId(accName);

                // UniqueId Found >> Save!
                if (File.Exists(Platform.IdsJsonPath))
                {
                    var allIds = ReadAllIds();
                    if (!string.IsNullOrEmpty(uniqueId) && allIds.ContainsKey(uniqueId))
                    {
                        if (accName == allIds[uniqueId])
                        {
                            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_AlreadyLoggedIn"], renderTo: "toastarea");
                            GeneralFuncs.StartProgram(Basic.Exe(), Basic.Admin, args);
                            return;
                        }
                        BasicAddCurrent(allIds[uniqueId]);
                    }
                }
            }

            // Clear current login
            ClearCurrentLoginBasic();

            // Copy saved files in
            if (accName != "")
            {
                if (!BasicCopyInAccount(accName)) return;
                Globals.AddTrayUser(Platform.SafeName, $"+{CurrentPlatform.Instance.PrimaryId}:" + accName, accName, Basic.TrayAccNumber); // Add to Tray list, using first Identifier
            }

            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_StartingPlatform", new { platform = Platform.FullName }]);
            GeneralFuncs.StartProgram(Basic.Exe(), Basic.Admin, args);

            Globals.RefreshTrayArea();
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
        }

        [SupportedOSPlatform("windows")]
        private static bool ClearCurrentLoginBasic()
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ClearCurrentLoginBasic]");

            foreach (var accFile in Platform.PathListToClear)
            {
                // The "file" is a registry key
                if (accFile.StartsWith("REG:"))
                {
                    if (!Globals.SetRegistryKey(accFile[3..])) // Remove "REG:" and read data
                    {
                        _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RegFailWrite"], Lang["Error"], "toastarea");
                        return false;
                    }
                    continue;
                }

                var path = Environment.ExpandEnvironmentVariables(accFile);
                if (!(File.Exists(path) || Directory.Exists(path))) continue;
                if (File.GetAttributes(Environment.ExpandEnvironmentVariables(path)).HasFlag(FileAttributes.Directory))
                {
                    Globals.RecursiveDelete(new DirectoryInfo(path), true);
                }
                else
                {
                    File.Delete(path);
                }
            }

            if (Platform.UniqueIdMethod != "CREATE_ID_FILE") return true;

            // Unique ID file --> This needs to be deleted for a new instance
            var uniqueIdFile = Platform.GetUniqueFilePath();
            if (File.Exists(uniqueIdFile)) File.Delete(uniqueIdFile);

            return true;
        }

        [SupportedOSPlatform("windows")]
        private static bool BasicCopyInAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.BasicCopyInAccount]");

            var localCachePath = Platform.AccountLoginCachePath(accName); ;
            _ = Directory.CreateDirectory(localCachePath);

            if (Platform.LoginFiles == null) throw new Exception("No data in basic platform: " + Platform.FullName);

            // Get unique ID from IDs file if unique ID is a registry key. Set if exists.
            if (Platform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(Platform.UniqueIdFile))
            {
                var uniqueId = GeneralFuncs.ReadAllIds_Generic(Platform.SafeName).FirstOrDefault(x => x.Value == accName).Key;

                if (!string.IsNullOrEmpty(uniqueId) && !Globals.SetRegistryKey(Platform.UniqueIdFile, uniqueId)) // Remove "REG:" and read data
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_AlreadyLoggedIn"], Lang["Error"], "toastarea");
                    return false;
                }
            }

            foreach (var (accFile, savedFile) in Platform.LoginFiles)
            {
                // The "file" is a registry key
                if (accFile.StartsWith("REG:"))
                {
                    var regFile = Path.Join(localCachePath, (string) Platform.LoginFiles[accFile]);
                    if (!File.Exists(regFile))
                    {
                        _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RegSaveMissing"], Lang["Error"], "toastarea");
                        return false;
                    }

                    if (!Globals.SetRegistryKey(accFile[3..], File.ReadAllText(regFile))) // Remove "REG:" and read data
                    {
                        _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RegFailWrite"], Lang["Error"], "toastarea");
                        return false;
                    }
                    continue;
                }

                // Check if it's a folder
                if (accFile.EndsWith('*'))
                {
                    var fld = accFile[..^1];
                    var localFld = Path.Join(localCachePath, savedFile);
                    Globals.CopyFilesRecursive(localFld, fld, true);
                    continue;
                }

                var saved = Path.Join(localCachePath, savedFile);

                if (File.Exists(saved))
                    File.Copy(saved, Environment.ExpandEnvironmentVariables(accFile), true);
            }

            return true;
        }

        [SupportedOSPlatform("windows")]
        public static bool BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.BasicAddCurrent]");
            var localCachePath = Platform.AccountLoginCachePath(accName);
            _ = Directory.CreateDirectory(localCachePath);

            if (Platform.LoginFiles == null) throw new Exception("No data in basic platform: " + Platform.FullName);

            var uniqueId = "";
            if (Platform.UniqueIdMethod is "REGKEY" && !string.IsNullOrEmpty(Platform.UniqueIdFile))
            {
                if (!ReadRegistryKeyWithErrors(Platform.UniqueIdFile, out uniqueId))
                    return false;
            }
            else
                uniqueId = GetUniqueId(accName);

            if (uniqueId == "" && Platform.UniqueIdMethod == "CREATE_ID_FILE")
            {
                // Unique ID file, and does not already exist: Therefore create!
                var uniqueIdFile = Platform.GetUniqueFilePath();
                uniqueId = Globals.RandomString(16);
                File.WriteAllText(uniqueIdFile, uniqueId);
            }

            foreach (var (accFile, savedFile) in Platform.LoginFiles)
            { // The "file" is a registry key
                if (accFile.StartsWith("REG:"))
                {
                    var trimmedName = accFile[3..];

                    if (ReadRegistryKeyWithErrors(trimmedName, out var response)) // Remove "REG:" and read data
                    {
                        // Write registry value to provided file
                        File.WriteAllText(Path.Join(localCachePath, (string)Platform.LoginFiles[accFile]), response);
                        continue;
                    }
                }

                // Check if it's a folder
                if (accFile.EndsWith('*'))
                {
                    var fld = accFile[..^1];
                    var localFld = Path.Join(localCachePath, savedFile);
                    if (!Directory.Exists(localFld))
                        Directory.CreateDirectory(localFld);
                    Globals.CopyFilesRecursive(fld, localFld, true);
                    continue;
                }


                if (!(Directory.Exists(accFile) || File.Exists(accFile)))
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotFindX", new { x = accFile }], Lang["DirectoryNotFound"], "toastarea");
                    return false;
                }

                // Run some action that can be specified in the BasicPlatforms.json file
                // Add for the start, and end of this function -- To allow 'plugins'?
                // Use reflection?

                if (File.Exists(Environment.ExpandEnvironmentVariables(accFile)))
                    File.Copy(Environment.ExpandEnvironmentVariables(accFile), Path.Join(localCachePath, savedFile), true);
            }

            var allIds = GeneralFuncs.ReadAllIds_Generic(Platform.IdsJsonPath);
            allIds[uniqueId] = accName;
            File.WriteAllText(Platform.IdsJsonPath, JsonConvert.SerializeObject(allIds));

            // Copy in profile image from default
            // TODO: Replace all Uri.EscapeDataString for images with Globals.GetCleanFilePath?
            _ = Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{Platform.SafeName}"));
            var profileImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{Platform.SafeName}\\{Globals.GetCleanFilePath(accName)}.jpg");
            if (!File.Exists(profileImg))
            {
                var platformImgPath = "\\img\\" + Platform.SafeName + ".png";
                File.Copy(
                    File.Exists(Platform.SafeName)
                        ? Path.Join(GeneralFuncs.WwwRoot(), platformImgPath)
                        : Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png"), profileImg, true);
            }

            AppData.ActiveNavMan?.NavigateTo("/Basic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_SavedItem", new { item = accName }]), true);
            return true;
        }

        public static string GetUniqueId(string accName, bool fromSaved = false)
        {
            var fileToRead = fromSaved ? Path.Join(Platform.AccountLoginCachePath(accName), Platform.UniqueIdFile) : Platform.GetUniqueFilePath();

            var uniqueId = "";

            if (Platform.UniqueIdMethod is "REGKEY")
            {
                _ = ReadRegistryKeyWithErrors(Platform.UniqueIdFile, out uniqueId);
                return uniqueId;
            }

            if (Platform.UniqueIdMethod is "CREATE_ID_FILE")
            {
                return File.Exists(fileToRead) ? File.ReadAllText(fileToRead) : uniqueId;
            }

            if (Platform.UniqueIdFile is not "" && File.Exists(fileToRead))
            {
                if (Platform.UniqueIdRegex != null)
                {
                    var m = Regex.Match(
                        File.ReadAllText(fileToRead),
                        Platform.UniqueIdRegex, RegexOptions.IgnoreCase);
                    if (m.Success)
                        uniqueId = m.Value;
                }
                else if (Platform.UniqueIdMethod is "FILE_MD5") // TODO: TEST THIS! -- This is used for static files that do not change throughout the lifetime of an account login.
                {
                    if (!Platform.UniqueIdFile.Contains('*')) uniqueId = GeneralFuncs.GetFileMd5(fileToRead);
                    else
                        uniqueId = string.Join('|', (from f in new DirectoryInfo(fileToRead).GetFiles()
                            where f.Name.EndsWith(Platform.UniqueIdFile.Split('*')[1])
                            select GeneralFuncs.GetFileMd5(f.FullName)).ToList());
                }
            }
            else if (uniqueId != "")
                uniqueId = Globals.GetSha256HashString(uniqueId);

            return uniqueId;
        }

        private static bool ReadRegistryKeyWithErrors(string key, out string value)
        {
            value = Globals.ReadRegistryKey(key);
            switch (value)
            {
                case "ERROR-NULL":
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_AccountIdReg"], Lang["Error"], "toastarea");
                    return false;
                case "ERROR-READ":
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RegFailRead"], Lang["Error"], "toastarea");
                    return false;
            }

            return true;
        }
        public static void ChangeUsername(string oldName, string newName, bool reload = true)
        {
            var allIds = GeneralFuncs.ReadAllIds_Generic(Platform.SafeName);
            try
            {
                allIds[allIds.Single(x => x.Value == oldName).Key] = newName;
                File.WriteAllText(Platform.IdsJsonPath, JsonConvert.SerializeObject(allIds));
            }
            catch (Exception)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_CantChangeUsername"], Lang["Error"], "toastarea");
                return;
            }

            File.Move(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{Platform.SafeName}\\{Uri.EscapeDataString(oldName)}.jpg"),
                Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{Platform.SafeName}\\{Uri.EscapeDataString(newName)}.jpg")); // Rename image
            Directory.Move($"LoginCache\\{Platform.SafeName}\\{oldName}\\", $"LoginCache\\{Platform.SafeName}\\{newName}\\"); // Rename login cache folder

            if (reload) AppData.ActiveNavMan?.NavigateTo("/Basic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_ChangedUsername"]), true);
        }

        private static Dictionary<string, string> ReadAllIds()
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ReadAllIds]");
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            if (!File.Exists(Platform.IdsJsonPath)) return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
            try
            {
                s = Globals.ReadAllText(Platform.IdsJsonPath);
            }
            catch (Exception)
            {
                //
            }

            return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }
    }
}
