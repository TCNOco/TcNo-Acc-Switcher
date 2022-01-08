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

        private static readonly Data.AppData AppData = Data.AppData.Instance;
        /*
                private static string _basicRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Basic");
        */
        private static readonly string BasicLocalAppData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "BasicGamesLauncher");
        private static readonly string BasicSavedConfigWin = Path.Join(BasicLocalAppData, "Saved\\Config\\Windows");
        private static readonly string BasicGameUserSettings = Path.Join(BasicSavedConfigWin, "GameUserSettings.ini");
        /// <summary>
        /// Main function for Basic Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static void LoadProfiles()
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.LoadProfiles] Loading Basic profiles for: " + AppData.BasicCurrentPlatform);
            Data.Settings.Basic.Instance.LoadFromFile();
            _ = GenericFunctions.GenericLoadAccounts(AppData.BasicCurrentPlatform);
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetBasicForgetAcc() => Task.FromResult(Basic.ForgetAccountEnabled);

        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="accName">Basic account name</param>
        public static bool ForgetAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:BasicBasicSwitcherFuncs.ForgetAccount] Forgetting account: hidden");
            // Remove ID from list of ids
            var allIds = ReadAllIds();
            _ = allIds.Remove(allIds.Single(x => x.Value == accName).Key);
            File.WriteAllText("LoginCache\\Basic\\ids.json", JsonConvert.SerializeObject(allIds));
            // Remove cached files
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Basic\\{accName}"), false);
            // Remove image
            var img = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\basic\\{Uri.EscapeDataString(accName)}.jpg");
            if (File.Exists(img)) File.Delete(img);
            // Remove from Tray
            Globals.RemoveTrayUser("Basic", accName); // Add to Tray list
            return true;
        }

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
            if (!GeneralFuncs.CloseProcesses(AppData.BasicCurrentPlatformProcesses, Data.Settings.Basic.Instance.AltClose))
            {
                _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = AppData.BasicCurrentPlatform }]);
                return;
            };

            // Clear current login
            ClearCurrentLoginBasic();

            // Copy saved files in
            if (accName != "")
            {
                if (!BasicCopyInAccount(accName)) return;
                Globals.AddTrayUser("Basic", $"+{AppData.BasicCurrentPlatformIds[0]}:" + accName, accName, Basic.TrayAccNumber); // Add to Tray list, using first Identifier
            }

            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_StartingPlatform", new { platform = AppData.BasicCurrentPlatform }]);
            GeneralFuncs.StartProgram(Basic.Exe(), Basic.Admin, args);

            Globals.RefreshTrayArea();
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
        }

        [SupportedOSPlatform("windows")]
        private static void ClearCurrentLoginBasic()
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ClearCurrentLoginBasic]");

            // Iterate through list of files/folders to delete/clear
            if (AppData.BasicCurrentPlatformJson["LoginFiles"] == null) throw new Exception("No data in basic platform: " + AppData.BasicCurrentPlatform);

            foreach (var jToken in (JToken)AppData.BasicCurrentPlatformJson["LoginFiles"])
            {
                var item = (JProperty)jToken;
                var path = Environment.ExpandEnvironmentVariables(item.Name);

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

            // TODO: Handle clearing Registry entries
        }

        [SupportedOSPlatform("windows")]
        private static bool BasicCopyInAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.BasicCopyInAccount]");

            var localCachePath = $"LoginCache\\{AppData.BasicCurrentPlatformSafeString}\\{accName}\\";
            _ = Directory.CreateDirectory(localCachePath);

            if (AppData.BasicCurrentPlatformJson["LoginFiles"] == null) throw new Exception("No data in basic platform: " + AppData.BasicCurrentPlatform);

            foreach (var jToken in (JToken)AppData.BasicCurrentPlatformJson["LoginFiles"])
            {
                var item = (JProperty)jToken;
                var saved = Path.Join(localCachePath, (string)item.Value);
                if (File.Exists(saved))
                    File.Copy(saved, Environment.ExpandEnvironmentVariables(item.Name), true);
            }

            // TODO: Handle registry

            return true;
        }

        [SupportedOSPlatform("windows")]
        public static void BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.BasicAddCurrent]");
            var localCachePath = $"LoginCache\\{AppData.BasicCurrentPlatformSafeString}\\{accName}\\";
            _ = Directory.CreateDirectory(localCachePath);

            if (AppData.BasicCurrentPlatformJson["LoginFiles"] == null) throw new Exception("No data in basic platform: " + AppData.BasicCurrentPlatform);

            foreach (JProperty item in (JToken)AppData.BasicCurrentPlatformJson["LoginFiles"])
            {
                if (File.Exists(Environment.ExpandEnvironmentVariables(item.Name)))
                    File.Copy(Environment.ExpandEnvironmentVariables(item.Name), Path.Join(localCachePath, (string)item.Value), true);
            }

            var uniqueId = "";
            // TODO: Registry alternative to this.
            if (AppData.BasicCurrentPlatformJson.ContainsKey("UniqueIdFile"))
            {
                var regexPattern = Globals.ExpandRegex((string) AppData.BasicCurrentPlatformJson["UniqueIdRegex"]);


                var m = Regex.Match(
                    File.ReadAllText(Path.Join($"LoginCache\\{AppData.BasicCurrentPlatformSafeString}\\{accName}\\",
                        (string) AppData.BasicCurrentPlatformJson["UniqueIdFile"])),
                    regexPattern, RegexOptions.IgnoreCase);
                if (m.Success)
                    uniqueId = m.Value;
            }
            else
                uniqueId = Globals.GetSha256HashString(uniqueId);

            var allIds = ReadAllIds();
            allIds[uniqueId] = accName;
            File.WriteAllText($"LoginCache\\{AppData.BasicCurrentPlatformSafeString}\\ids.json", JsonConvert.SerializeObject(allIds));

            // Copy in profile image from default
            // TODO: Replace all Uri.EscapeDataString for images with Globals.GetCleanFilePath?
            _ = Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.BasicCurrentPlatformSafeString}"));
            var profileImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.BasicCurrentPlatformSafeString}\\{Globals.GetCleanFilePath(accName)}.jpg");
            if (!File.Exists(profileImg))
            {
                var platformImgPath = "\\img\\" + AppData.BasicCurrentPlatformSafeString + ".png";
                File.Copy(
                    File.Exists(AppData.BasicCurrentPlatformSafeString)
                        ? Path.Join(GeneralFuncs.WwwRoot(), platformImgPath)
                        : Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png"), profileImg, true);
            }

            AppData.ActiveNavMan?.NavigateTo("/Basic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_SavedItem", new { item = accName }]), true);
        }

        public static void ChangeUsername(string oldName, string newName, bool reload = true)
        {
            var allIds = ReadAllIds();
            try
            {
                allIds[allIds.Single(x => x.Value == oldName).Key] = newName;
                File.WriteAllText("LoginCache\\Basic\\ids.json", JsonConvert.SerializeObject(allIds));
            }
            catch (Exception)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_CantChangeUsername"], Lang["Error"], "toastarea");
                return;
            }

            File.Move(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\basic\\{Uri.EscapeDataString(oldName)}.jpg"),
                Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\basic\\{Uri.EscapeDataString(newName)}.jpg")); // Rename image
            Directory.Move($"LoginCache\\Basic\\{oldName}\\", $"LoginCache\\Basic\\{newName}\\"); // Rename login cache folder

            if (reload) AppData.ActiveNavMan?.NavigateTo("/Basic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_ChangedUsername"]), true);
        }

        private static Dictionary<string, string> ReadAllIds()
        {
            Globals.DebugWriteLine(@"[Func:Basic\BasicSwitcherFuncs.ReadAllIds]");
            const string localAllIds = "LoginCache\\Basic\\ids.json";
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            if (!File.Exists(localAllIds)) return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
            try
            {
                s = Globals.ReadAllText(localAllIds);
            }
            catch (Exception)
            {
                //
            }

            return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }
    }
}
