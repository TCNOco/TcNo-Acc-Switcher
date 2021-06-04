using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Xml;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Origin
{
    public class OriginSwitcherFuncs
    {
        private static readonly Data.Settings.Origin Origin = Data.Settings.Origin.Instance;
        private static readonly string OriginRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Origin");
        private static readonly string OriginProgramData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Origin");
        /// <summary>
        /// Main function for Origin Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async void LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine(@"[Func:Origin\OriginSwitcherFuncs.LoadProfiles] Loading Origin profiles");
            
            var localCachePath = "LoginCache\\Origin\\";
            if (!Directory.Exists(localCachePath)) return;
            var accList = new List<string>();
            foreach (var f in Directory.GetDirectories(localCachePath))
            {
                var lastSlash = f.LastIndexOf("\\", StringComparison.Ordinal) + 1;
                var accName = f.Substring(lastSlash, f.Length - lastSlash);
                accList.Add(accName);
            }

            // Order
            if (File.Exists("LoginCache\\Origin\\order.json"))
            {
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\Origin\\order.json"));
                if (savedOrder != null)
                {
                    var index = 0;
                    if (savedOrder is { Count: > 0 })
                        foreach (var acc in from i in savedOrder where accList.Any(x => x == i) select accList.Single(x => x == i))
                        {
                            accList.Remove(acc);
                            accList.Insert(index, acc);
                            index++;
                        }
                }
            }

            foreach (var element in 
                accList.Select(accName => 
                    $"<div class=\"acc_list_item\"><input type=\"radio\" id=\"{accName}\" DisplayName=\"{accName}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{accName}\" class=\"acc\">\r\n" +
                    $"<img src=\"\\img\\profiles\\origin\\{Uri.EscapeUriString(accName)}.jpg\" draggable=\"false\" />\r\n" +
                    $"<h6>{accName}</h6></div>\r\n"))
                await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", "#acc_list", element);

            _ = AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryProcessAccListSize");
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initAccListSortable");
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetOriginForgetAcc() => Task.FromResult(Origin.ForgetAccountEnabled);

        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="accName">Origin account name</param>
        public static bool ForgetAccount(string accName)
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.ForgetAccount] Forgetting account: {accName}");
            // Remove cached files
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Origin\\{accName}"), false);
            // Remove image
            var img = Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\{Uri.EscapeUriString(accName)}.jpg");
            if (File.Exists(img)) File.Delete(img);
            // Remove from Tray
            Globals.RemoveTrayUser("Origin", accName); // Add to Tray list
            return true;
        }

        /// <summary>
        /// Restart Origin with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        /// <param name="state">(Optional) 10 = Invisible, 0 = Default</param>
        public static void SwapOriginAccounts(string accName = "", int state = 0)
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.SwapOriginAccounts] Swapping to: {accName}.");
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Closing Origin");
            if (!CloseOrigin()) return;
            // DO ACTUAL SWITCHING HERE
            ClearCurrentLoginOrigin();
            if (accName != "")
            {
                OriginCopyInAccount(accName, state);
                Globals.AddTrayUser("Origin", "+o:" + accName, accName, Origin.TrayAccNumber); // Add to Tray list
            }
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting Origin");
            
            GeneralFuncs.StartProgram(Origin.Exe(), Origin.Admin);

            Globals.RefreshTrayArea();
        }


        private static void ClearCurrentLoginOrigin()
        {
            Globals.DebugWriteLine(@"[Func:Origin\OriginSwitcherFuncs.ClearCurrentLoginOrigin]");
            // Get current information for logged in user, and save into files:
            var currentOlcHashes = (from f in new DirectoryInfo(OriginProgramData).GetFiles() where f.Name.EndsWith(".olc") select GeneralFuncs.GetFileMd5(f.FullName)).ToList();

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
                OriginAddCurrent(activeAccount);
            }

            // Clear for next login
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(OriginRoaming, "ConsolidatedCache")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(OriginRoaming, "NucleusCache")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(OriginRoaming, "Logs")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(OriginProgramData, "Subscription")), false);
            Directory.CreateDirectory(Path.Join(OriginProgramData, "Subscription"));

            foreach (var f in new DirectoryInfo(OriginRoaming).GetFiles())
            {
                if (f.Name.StartsWith("local") && f.Name.EndsWith(".xml"))
                {
                    File.Delete(f.FullName);
                }
            }
            foreach (var f in new DirectoryInfo(OriginProgramData).GetFiles())
            {
                if (f.Name.EndsWith(".xml") || f.Name.EndsWith(".olc"))
                {
                    File.Delete(f.FullName);
                }
            }
            File.WriteAllText(OriginProgramData + "\\local.xml", "<?xml version=\"1.0\"?><Settings><Setting key=\"OfflineLoginUrl\" value=\"https://signin.ea.com/p/originX/offline\" type=\"10\"/></Settings>");
        }

        private static void OriginCopyInAccount(string accName, int state = 0)
        {
            Globals.DebugWriteLine(@"[Func:Origin\OriginSwitcherFuncs.OriginCopyInAccount]");
            var localCachePath = $"LoginCache\\Origin\\{accName}\\";
            var localCachePathData = $"LoginCache\\Origin\\{accName}\\Data\\";

            GeneralFuncs.CopyFilesRecursive($"{localCachePath}ConsolidatedCache", Path.Join(OriginRoaming, "ConsolidatedCache"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePath}NucleusCache", Path.Join(OriginRoaming, "NucleusCache"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePath}Logs", Path.Join(OriginRoaming, "Logs"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePathData}Subscription", Path.Join(OriginProgramData, "Subscription"));

            foreach (var f in new DirectoryInfo(localCachePath).GetFiles())
            {
                if (!f.Name.StartsWith("local") || !f.Name.EndsWith(".xml")) continue;
                File.Copy(f.FullName, Path.Join(OriginRoaming, f.Name), true);
                if (f.Name == "local.xml") continue;
                //LoginAsInvisible
                var profileXml = new XmlDocument();
                profileXml.Load(f.FullName);
                if (profileXml.DocumentElement == null) continue;
                foreach (XmlNode n in profileXml.DocumentElement.SelectNodes("/Settings/Setting"))
                {
                    if (n.Attributes?["key"]?.Value != "LoginAsInvisible") continue;
                    n.Attributes["value"].Value = (state == 10 ? "true" : "false");
                    profileXml.Save(Path.Join(OriginRoaming, f.Name));
                    break;
                }
            }

            foreach (var f in new DirectoryInfo(localCachePathData).GetFiles())
            {
                if (f.Name.EndsWith(".xml") || f.Name.EndsWith(".olc"))
                {
                    File.Copy(f.FullName, Path.Join(OriginProgramData, f.Name), true);
                }
            }
        }

        public static void OriginAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Origin\OriginSwitcherFuncs.OriginAddCurrent]");
            var localCachePath = $"LoginCache\\Origin\\{accName}\\";
            var localCachePathData = $"LoginCache\\Origin\\{accName}\\Data\\";
            Directory.CreateDirectory(localCachePath);
            
            GeneralFuncs.CopyFilesRecursive(Path.Join(OriginRoaming, "ConsolidatedCache"), $"{localCachePath}ConsolidatedCache");
            GeneralFuncs.CopyFilesRecursive(Path.Join(OriginRoaming, "NucleusCache"), $"{localCachePath}NucleusCache");
            GeneralFuncs.CopyFilesRecursive(Path.Join(OriginRoaming, "Logs"), $"{localCachePath}Logs");
            GeneralFuncs.CopyFilesRecursive(Path.Join(OriginProgramData, "Subscription"), $"{localCachePathData}Subscription");

            var pfpFilePath = "";
            foreach (var f in new DirectoryInfo(OriginRoaming).GetFiles())
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

            Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot, "\\img\\profiles\\origin\\"));
            var destImg = Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\{Uri.EscapeUriString(accName)}.jpg");
            if (File.Exists(destImg))
            {
                GeneralFuncs.DeletedOutdatedFile(destImg, Origin.ImageExpiryTime);
                GeneralFuncs.DeletedInvalidImage(destImg);
            }
            if (!File.Exists(destImg)) File.Copy((pfpFilePath != "" ? pfpFilePath :  Path.Join(GeneralFuncs.WwwRoot,"img\\QuestionMark.jpg"))!, destImg, true);

            var olcHashes = new List<string>();
            foreach (var f in new DirectoryInfo(OriginProgramData).GetFiles())
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
            File.WriteAllText("LoginCache\\Origin\\olc.json", JsonConvert.SerializeObject(allOlc));
            AppData.ActiveNavMan?.NavigateTo("/Origin/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + accName), true);
        }

        public static void ChangeUsername(string oldName, string newName, bool reload = false)
        {
            var allOlc = ReadAllOlc();
            if (!ChangeKey(ref allOlc, oldName, newName)) // Rename account in olc.json
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not change username", "Error", "toastarea");
                return;
            }
            File.WriteAllText("LoginCache\\Origin\\olc.json", JsonConvert.SerializeObject(allOlc));

            File.Move(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\{Uri.EscapeUriString(oldName)}.jpg"),
                Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\{Uri.EscapeUriString(newName)}.jpg")); // Rename image
            Directory.Move($"LoginCache\\Origin\\{oldName}\\", $"LoginCache\\Origin\\{newName}\\"); // Rename login cache folder

            if (reload) AppData.ActiveNavMan?.NavigateTo("/Origin/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Changed username"), true);
        }

        public static bool ChangeKey<TKey, TValue>(ref Dictionary<TKey, TValue> dict, TKey oldKey, TKey newKey)
        {
            if (!dict.Remove(oldKey, out var value))
                return false;

            dict[newKey] = value;
            return true;
        }

        private static Dictionary<string, List<string>> ReadAllOlc()
        {
            Globals.DebugWriteLine(@"[Func:Origin\OriginSwitcherFuncs.ReadAllOlc]");
            var localAllOlc = "LoginCache\\Origin\\olc.json";
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


        #region ORIGIN_MANAGEMENT
        /// <summary>
        /// Kills Origin processes when run via cmd.exe
        /// </summary>
        public static bool CloseOrigin()
        {
            Globals.DebugWriteLine(@"[Func:Origin\OriginSwitcherFuncs.CloseOrigin]");
            if (!GeneralFuncs.CanKillProcess("origin")) return false;
            Globals.KillProcess("origin");
            return true;
        }
        #endregion
    }
}
