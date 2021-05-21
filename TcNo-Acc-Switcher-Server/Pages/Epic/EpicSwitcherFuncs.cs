using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using Microsoft.JSInterop;
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
        private static string _epicProgramData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Epic");
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
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Epic\\{accName}"), false);
            return true;
        }

        /// <summary>
        /// Restart Epic with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        /// <param name="state">(Optional) 10 = Invisible, 0 = Default</param>
        public static void SwapEpicAccounts(string accName = "", int state = 0)
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.SwapEpicAccounts] Swapping to: {accName}.");
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Closing Epic");
            if (!CloseEpic()) return;
            // DO ACTUAL SWITCHING HERE
            ClearCurrentLoginEpic();
            if (accName != "")
            {
                EpicCopyInAccount(accName, state);
            }
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting Epic");

            Globals.AddTrayUser("Epic", "+o:" + accName, accName, Epic.TrayAccNumber); // Add to Tray list

            GeneralFuncs.StartProgram(Epic.Exe(), Epic.Admin);
        }


        private static void ClearCurrentLoginEpic()
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.ClearCurrentLoginEpic]");
            // Get current information for logged in user, and save into files:
            var currentOlcHashes = (from f in new DirectoryInfo(_epicProgramData).GetFiles() where f.Name.EndsWith(".olc") select GeneralFuncs.GetFileMd5(f.FullName)).ToList();

            var activeAccount = "";
            var allOlc = ReadAllOlc();
            foreach (var (key, val) in allOlc)
            {
                if (activeAccount != "") break;
                if (currentOlcHashes.Any(cH => val.Contains(cH)))
                {
                    activeAccount = key;
                }
            }
            // Copy files in:
            if (activeAccount != "")
            {
                EpicAddCurrent(activeAccount);
            }

            // Clear for next login
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_epicRoaming, "ConsolidatedCache")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_epicRoaming, "NucleusCache")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_epicRoaming, "Logs")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_epicProgramData, "Subscription")), false);
            Directory.CreateDirectory(Path.Join(_epicProgramData, "Subscription"));

            foreach (var f in new DirectoryInfo(_epicRoaming).GetFiles())
            {
                if (f.Name.StartsWith("local") && f.Name.EndsWith(".xml"))
                {
                    File.Delete(f.FullName);
                }
            }
            foreach (var f in new DirectoryInfo(_epicProgramData).GetFiles())
            {
                if (f.Name.EndsWith(".xml") || f.Name.EndsWith(".olc"))
                {
                    File.Delete(f.FullName);
                }
            }
            File.WriteAllText(_epicProgramData + "\\local.xml", "<?xml version=\"1.0\"?><Settings><Setting key=\"OfflineLoginUrl\" value=\"https://signin.ea.com/p/originX/offline\" type=\"10\"/></Settings>");
        }

        private static void EpicCopyInAccount(string accName, int state = 0)
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.EpicCopyInAccount]");
            var localCachePath = $"LoginCache\\Epic\\{accName}\\";
            var localCachePathData = $"LoginCache\\Epic\\{accName}\\Data\\";

            GeneralFuncs.CopyFilesRecursive($"{localCachePath}ConsolidatedCache", Path.Join(_epicRoaming, "ConsolidatedCache"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePath}NucleusCache", Path.Join(_epicRoaming, "NucleusCache"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePath}Logs", Path.Join(_epicRoaming, "Logs"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePathData}Subscription", Path.Join(_epicProgramData, "Subscription"));

            foreach (var f in new DirectoryInfo(localCachePath).GetFiles())
            {
                if (!f.Name.StartsWith("local") || !f.Name.EndsWith(".xml")) continue;
                File.Copy(f.FullName, Path.Join(_epicRoaming, f.Name), true);
                if (f.Name == "local.xml") continue;
                //LoginAsInvisible
                var profileXml = new XmlDocument();
                profileXml.Load(f.FullName);
                if (profileXml.DocumentElement == null) continue;
                foreach (XmlNode n in profileXml.DocumentElement.SelectNodes("/Settings/Setting"))
                {
                    if (n.Attributes?["key"]?.Value != "LoginAsInvisible") continue;
                    n.Attributes["value"].Value = (state == 10 ? "true" : "false");
                    profileXml.Save(Path.Join(_epicRoaming, f.Name));
                    break;
                }
            }

            foreach (var f in new DirectoryInfo(localCachePathData).GetFiles())
            {
                if (f.Name.EndsWith(".xml") || f.Name.EndsWith(".olc"))
                {
                    File.Copy(f.FullName, Path.Join(_epicProgramData, f.Name), true);
                }
            }
        }

        public static void EpicAddCurrent(string accName)
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.EpicAddCurrent]");
            var localCachePath = $"LoginCache\\Epic\\{accName}\\";
            var localCachePathData = $"LoginCache\\Epic\\{accName}\\Data\\";
            Directory.CreateDirectory(localCachePath);
            
            GeneralFuncs.CopyFilesRecursive(Path.Join(_epicRoaming, "ConsolidatedCache"), $"{localCachePath}ConsolidatedCache");
            GeneralFuncs.CopyFilesRecursive(Path.Join(_epicRoaming, "NucleusCache"), $"{localCachePath}NucleusCache");
            GeneralFuncs.CopyFilesRecursive(Path.Join(_epicRoaming, "Logs"), $"{localCachePath}Logs");
            GeneralFuncs.CopyFilesRecursive(Path.Join(_epicProgramData, "Subscription"), $"{localCachePathData}Subscription");

            var pfpFilePath = "";
            foreach (var f in new DirectoryInfo(_epicRoaming).GetFiles())
            {
                if (f.Name.StartsWith("local") && f.Name.EndsWith(".xml"))
                {
                    File.Copy(f.FullName, $"{localCachePath}{f.Name}", true);
                }

                if (f.Name == "local.xml") continue;
                if (pfpFilePath != "") continue;
                // Handle profile picture (Unfortunately the low-res one. No idea how to find the file name of the full one...)
                var profileXml = new XmlDocument();
                profileXml.Load(f.FullName);
                if (profileXml.DocumentElement == null) continue;
                foreach (XmlNode n in profileXml.DocumentElement.SelectNodes("/Settings/Setting"))
                {
                    if (n.Attributes?["key"]?.Value != "UserAvatarCacheURL") continue;
                    pfpFilePath = n.Attributes["value"]?.Value;
                    break;
                }
            }

            Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\epic\\"));
            File.Copy((pfpFilePath != "" ? pfpFilePath :  Path.Join(GeneralFuncs.WwwRoot,"img\\QuestionMark.jpg"))!, Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\epic\\{Uri.EscapeUriString(accName)}.jpg"), true);

            var olcHashes = new List<string>();
            foreach (var f in new DirectoryInfo(_epicProgramData).GetFiles())
            {
                if (f.Name.EndsWith(".xml"))
                {
                    File.Copy(f.FullName, $"{localCachePathData}{f.Name}", true);
                }else if (f.Name.EndsWith(".olc"))
                {
                    File.Copy(f.FullName, $"{localCachePathData}{f.Name}", true);
                    olcHashes.Add(GeneralFuncs.GetFileMd5(f.FullName)); // Add hashes to list
                }
            }

            var allOlc = ReadAllOlc();
            allOlc[accName] = olcHashes;
            File.WriteAllText("LoginCache\\Epic\\olc.json", JsonConvert.SerializeObject(allOlc));
            AppData.ActiveNavMan?.NavigateTo("/Epic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + accName), true);
        }

        public static void ChangeUsername(string oldName, string newName, bool reload = false)
        {
            var allOlc = ReadAllOlc();
            if (!ChangeKey(ref allOlc, oldName, newName)) // Rename account in olc.json
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not change username", "Error", "toastarea");
                return;
            }
            File.WriteAllText("LoginCache\\Epic\\olc.json", JsonConvert.SerializeObject(allOlc));

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

        private static Dictionary<string, List<string>> ReadAllOlc()
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.ReadAllOlc]");
            var localAllOlc = $"LoginCache\\Epic\\olc.json";
            var s = JsonConvert.SerializeObject(new Dictionary<string, List<string>>());
            if (File.Exists(localAllOlc))
            {
                try
                {
                     s = File.ReadAllText(localAllOlc);
                }
                catch (Exception)
                {
                    //
                }
            }

            return JsonConvert.DeserializeObject<Dictionary<string, List<string>>>(s);
        }


        #region EPIC_MANAGEMENT
        /// <summary>
        /// Kills Epic processes when run via cmd.exe
        /// </summary>
        public static bool CloseEpic()
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.CloseSteam]");
            if (!GeneralFuncs.CanKillProcess("epic")) return false;
            Globals.KillProcess("epic");
            return true;
        }
        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        /// <returns>Whether files were deleted or not</returns>
        public static async void ClearImages()
        {
            Globals.DebugWriteLine($@"[Func:Epic\EpicSwitcherFuncs.ClearImages] Clearing images.");
            //if (!Directory.Exists(Steam.SteamImagePath))
            //{
            //    await GeneralInvocableFuncs.ShowToast("error", "Could not clear images", "Error", "toastarea");
            //}
            //foreach (var file in Directory.GetFiles(Epic.EpicImage))
            //{
            //    File.Delete(file);
            //}
            //// Reload page, then display notification using a new thread.
            AppData.ActiveNavMan?.NavigateTo("/Epic/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared images"), true);
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
            var allOlc = ReadAllOlc();
            allOlc.Remove(accName);
            var olcHashString = JsonConvert.SerializeObject(allOlc);
            File.WriteAllText("LoginCache\\Epic\\olc.json", olcHashString);
            return Task.FromResult(ForgetAccount(accName));
        }

        #endregion
    }
}
