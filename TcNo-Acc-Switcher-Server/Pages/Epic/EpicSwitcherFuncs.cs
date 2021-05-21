using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using Formatting = System.Xml.Formatting;

namespace TcNo_Acc_Switcher_Server.Pages.Epic
{
    public class EpicSwitcherFuncs
    {
        private static readonly Data.Settings.Epic Epic = Data.Settings.Epic.Instance;
        private static string _epicRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Epic");
        private static string _epicLocalAppData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "EpicGamesLauncher");
        private static string _epicSavedConfigWin = Path.Join(_epicLocalAppData, "Saved\\Config\\Windows");
        private static string _epicGameUserSettings = Path.Join(_epicSavedConfigWin, "GameUserSettings.ini");
        /// <summary>
        /// Main function for Epic Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async void LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.LoadProfiles] Loading Epic profiles");
            
            var localCachePath = $"LoginCache\\Epic\\";
            if (!Directory.Exists(localCachePath)) return;
            var accList = new List<string>();
            foreach (var f in Directory.GetDirectories(localCachePath))
            {
                var lastSlash = f.LastIndexOf("\\", StringComparison.Ordinal) + 1;
                var accName = f.Substring(lastSlash, f.Length - lastSlash);
                accList.Add(accName);
            }

            // Order
            if (File.Exists("LoginCache\\Epic\\order.json"))
            {
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\Epic\\order.json"));
                var index = 0;
                if (savedOrder != null && savedOrder.Count > 0)
                    foreach (var acc in from i in savedOrder where accList.Any(x => x == i) select accList.Single(x => x == i))
                    {
                        accList.Remove(acc);
                        accList.Insert(index, acc);
                        index++;
                    }
            }

            foreach (var element in 
                accList.Select(accName => 
                    $"<div class=\"acc_list_item\"><input type=\"radio\" id=\"{accName}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{accName}\" class=\"acc\">\r\n" +
                    $"<img src=\"\\img\\profiles\\epic\\{Uri.EscapeUriString(accName)}.jpg\" draggable=\"false\" />\r\n" +
                    $"<h6>{accName}</h6></div>\r\n"))
                await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element });

            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initAccListSortable");
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetEpicForgetAcc() => Task.FromResult(Epic.ForgetAccountEnabled);

        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="accName">Epic account name</param>
        public static bool ForgetAccount(string accName)
        {
            Globals.DebugWriteLine($@"[Func:EpicEpicSwitcherFuncs.ForgetAccount] Forgetting account: {accName}");
            var allIds = ReadAllIds();
            allIds.Remove(allIds.Single(x => x.Value == accName).Key);
            File.WriteAllText("LoginCache\\Epic\\ids.json", JsonConvert.SerializeObject(allIds));
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Epic\\{accName}"), false);
            return true;
        }

        /// <summary>
        /// Restart Epic with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        [SupportedOSPlatform("windows")]
        public static void SwapEpicAccounts(string accName = "")
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.SwapEpicAccounts] Swapping to: {accName}.");
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Closing Epic");
            if (!CloseEpic()) return;
            // DO ACTUAL SWITCHING HERE
            ClearCurrentLoginEpic();
            if (accName != "")
            {
                EpicCopyInAccount(accName);
            }
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting Epic");

            Globals.AddTrayUser("Epic", "+o:" + accName, accName, Epic.TrayAccNumber); // Add to Tray list

            GeneralFuncs.StartProgram(Epic.Exe(), Epic.Admin);

            Globals.RefreshTrayArea();
        }

        [SupportedOSPlatform("windows")]
        private static void ClearCurrentLoginEpic()
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.ClearCurrentLoginEpic]");
            // Get current information for logged in user, and save into files:
            var currentAccountId = (string)Registry.CurrentUser.OpenSubKey(@"Software\Epic Games\Unreal Engine\Identifiers")?.GetValue("AccountId");

            var allIds = ReadAllIds();
            if (currentAccountId != null && allIds.ContainsKey(currentAccountId))
                EpicAddCurrent(allIds[currentAccountId]);
            
            if (File.Exists(_epicGameUserSettings)) File.Delete(_epicGameUserSettings); // Delete GameUserSettings.ini file
            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Epic Games\Unreal Engine\Identifiers");
            key.SetValue("AccountId", ""); // Clear logged in account in registry, but leave MachineId
        }

        [SupportedOSPlatform("windows")]
        private static void EpicCopyInAccount(string accName)
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.EpicCopyInAccount]");
            var localCachePath = $"LoginCache\\Epic\\{accName}\\";

            File.Copy(Path.Join(localCachePath, "GameUserSettings.ini"), _epicGameUserSettings);

            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Epic Games\Unreal Engine\Identifiers");

            var allIds = ReadAllIds();
            key.SetValue("AccountId", allIds.Single(x => x.Value == accName).Key);
        }

        [SupportedOSPlatform("windows")]
        public static void EpicAddCurrent(string accName)
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.EpicAddCurrent]");
            var localCachePath = $"LoginCache\\Epic\\{accName}\\";
            Directory.CreateDirectory(localCachePath);

            if (!File.Exists(_epicGameUserSettings))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not locate logged in user");
                return;
            }
            // Save files
            File.Copy(_epicGameUserSettings, Path.Join(localCachePath, "GameUserSettings.ini"), true);
            // Save registry key
            var currentAccountId = (string)Registry.CurrentUser.OpenSubKey(@"Software\Epic Games\Unreal Engine\Identifiers")?.GetValue("AccountId");
            if (currentAccountId == null)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Failed to get AccountId from Registry!");
                return;
            }

            var allIds = ReadAllIds();
            allIds[currentAccountId] = accName;
            File.WriteAllText("LoginCache\\Epic\\ids.json", JsonConvert.SerializeObject(allIds));

            // Copy in profile image from default
            Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\epic"));
            File.Copy(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\EpicDefault.png"), Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\epic\\{Uri.EscapeUriString(accName)}.jpg"), true);
            
            AppData.ActiveNavMan?.NavigateTo("/Epic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + accName), true);
        }

        public static void ChangeUsername(string oldName, string newName, bool reload = true)
        {
            var allIds = ReadAllIds();
            try
            {
                allIds[allIds.Single(x => x.Value == oldName).Key] = newName;
                File.WriteAllText("LoginCache\\Epic\\ids.json", JsonConvert.SerializeObject(allIds));
            }
            catch (Exception e)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not change username", "Error", "toastarea");
                return;
            }
            
            File.Move(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\epic\\{Uri.EscapeUriString(oldName)}.jpg"),
                Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\epic\\{Uri.EscapeUriString(newName)}.jpg")); // Rename image
            Directory.Move($"LoginCache\\Epic\\{oldName}\\", $"LoginCache\\Epic\\{newName}\\"); // Rename login cache folder

            if (reload) AppData.ActiveNavMan?.NavigateTo("/Epic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Changed username"), true);
        }

        public static bool ChangeKey<TKey, TValue>(ref Dictionary<TKey, TValue> dict, TKey oldKey, TKey newKey)
        {
            TValue value;
            if (!dict.Remove(oldKey, out value))
                return false;

            dict[newKey] = value;
            return true;
        }

        private static Dictionary<string, string> ReadAllIds()
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.ReadAllIds]");
            var localAllIds = $"LoginCache\\Epic\\ids.json";
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            if (File.Exists(localAllIds))
            {
                try
                {
                     s = File.ReadAllText(localAllIds);
                }
                catch (Exception)
                {
                    //
                }
            }

            return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }


        #region EPIC_MANAGEMENT
        /// <summary>
        /// Kills Epic processes when run via cmd.exe
        /// </summary>
        public static bool CloseEpic()
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.CloseSteam]");
            if (!GeneralFuncs.CanKillProcess("EpicGamesLauncher")) return false;
            Globals.KillProcess("EpicGamesLauncher");
            return true;
        }

        /// <summary>
        /// Only runs ForgetAccount, but allows Javascript to wait for it's completion before refreshing, instead of just doing it instantly >> Not showing proper results.
        /// </summary>
        /// <param name="accName">Epic account name to be removed from cache</param>
        /// <returns>true</returns>
        [JSInvokable]
        public static Task<bool> ForgetEpicAccountJs(string accName)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Epic\EpicSwitcherFuncs.ForgetEpicAccountJs] accName:{accName}");

            var allIds = ReadAllIds();
            allIds.Remove(allIds.Single(x => x.Value == accName).Key);
            File.WriteAllText("LoginCache\\Epic\\ids.json", JsonConvert.SerializeObject(allIds));
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Epic\\{accName}"), false);

            return Task.FromResult(ForgetAccount(accName));
        }

        #endregion
    }
}
