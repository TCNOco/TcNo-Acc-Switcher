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

namespace TcNo_Acc_Switcher_Server.Pages.Origin
{
    public class OriginSwitcherFuncs
    {
        private static readonly Data.Settings.Origin Origin = Data.Settings.Origin.Instance;
        private static string _originRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Origin");
        private static string _originProgramData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Origin");
        /// <summary>
        /// Main function for Origin Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async void LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.LoadProfiles] Loading Origin profiles");


            var localCachePath = $"LoginCache\\Origin\\";
            if (!Directory.Exists(localCachePath)) return;
            foreach (var f in Directory.GetDirectories(localCachePath))
            {
                var lastSlash = f.LastIndexOf("\\", StringComparison.Ordinal) + 1;
                var accName = f.Substring(lastSlash, f.Length - lastSlash);
                var element =
                    $"<input type=\"radio\" id=\"{accName}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{accName}\" class=\"acc\">\r\n" +
                    $"<img src=\"" + $"\\img\\profiles\\origin\\{Uri.EscapeUriString(accName)}.jpg" + "\" draggable=\"false\" />\r\n" +
                    $"<h6>{accName}</h6>\r\n";
                //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element });
                Console.WriteLine(f);
            }
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
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
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Origin\\{accName}"), false);
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
            }
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting Origin");

            Globals.AddTrayUser("Origin", "+o:" + accName, accName, Origin.TrayAccNumber); // Add to Tray list

            GeneralFuncs.StartProgram(Origin.Exe(), Origin.Admin);
        }


        private static void ClearCurrentLoginOrigin()
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.ClearCurrentLoginOrigin]");
            // Get current information for logged in user, and save into files:
            var currentOlcHashes = (from f in new DirectoryInfo(_originProgramData).GetFiles() where f.Name.EndsWith(".olc") select GeneralFuncs.GetFileMd5(f.FullName)).ToList();

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
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_originRoaming, "ConsolidatedCache")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_originRoaming, "NucleusCache")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_originRoaming, "Logs")), true);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(_originProgramData, "Subscription")), false);
            Directory.CreateDirectory(Path.Join(_originProgramData, "Subscription"));

            foreach (var f in new DirectoryInfo(_originRoaming).GetFiles())
            {
                if (f.Name.StartsWith("local") && f.Name.EndsWith(".xml"))
                {
                    File.Delete(f.FullName);
                }
            }
            foreach (var f in new DirectoryInfo(_originProgramData).GetFiles())
            {
                if (f.Name.EndsWith(".xml") || f.Name.EndsWith(".olc"))
                {
                    File.Delete(f.FullName);
                }
            }
            File.WriteAllText(_originProgramData + "\\local.xml", "<?xml version=\"1.0\"?><Settings><Setting key=\"OfflineLoginUrl\" value=\"https://signin.ea.com/p/originX/offline\" type=\"10\"/></Settings>");
        }

        private static void OriginCopyInAccount(string accName, int state = 0)
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.OriginCopyInAccount]");
            var localCachePath = $"LoginCache\\Origin\\{accName}\\";
            var localCachePathData = $"LoginCache\\Origin\\{accName}\\Data\\";

            GeneralFuncs.CopyFilesRecursive($"{localCachePath}ConsolidatedCache", Path.Join(_originRoaming, "ConsolidatedCache"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePath}NucleusCache", Path.Join(_originRoaming, "NucleusCache"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePath}Logs", Path.Join(_originRoaming, "Logs"));
            GeneralFuncs.CopyFilesRecursive($"{localCachePathData}Subscription", Path.Join(_originProgramData, "Subscription"));

            foreach (var f in new DirectoryInfo(localCachePath).GetFiles())
            {
                if (!f.Name.StartsWith("local") || !f.Name.EndsWith(".xml")) continue;
                File.Copy(f.FullName, Path.Join(_originRoaming, f.Name), true);
                if (f.Name == "local.xml") continue;
                //LoginAsInvisible
                var profileXml = new XmlDocument();
                profileXml.Load(f.FullName);
                if (profileXml.DocumentElement == null) continue;
                foreach (XmlNode n in profileXml.DocumentElement.SelectNodes("/Settings/Setting"))
                {
                    if (n.Attributes?["key"]?.Value != "LoginAsInvisible") continue;
                    n.Attributes["value"].Value = (state == 10 ? "true" : "false");
                    profileXml.Save(Path.Join(_originRoaming, f.Name));
                    break;
                }
            }

            foreach (var f in new DirectoryInfo(localCachePathData).GetFiles())
            {
                if (f.Name.EndsWith(".xml") || f.Name.EndsWith(".olc"))
                {
                    File.Copy(f.FullName, Path.Join(_originProgramData, f.Name), true);
                }
            }
        }

        public static void OriginAddCurrent(string accName)
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.OriginAddCurrent]");
            var localCachePath = $"LoginCache\\Origin\\{accName}\\";
            var localCachePathData = $"LoginCache\\Origin\\{accName}\\Data\\";
            Directory.CreateDirectory(localCachePath);
            
            GeneralFuncs.CopyFilesRecursive(Path.Join(_originRoaming, "ConsolidatedCache"), $"{localCachePath}ConsolidatedCache");
            GeneralFuncs.CopyFilesRecursive(Path.Join(_originRoaming, "NucleusCache"), $"{localCachePath}NucleusCache");
            GeneralFuncs.CopyFilesRecursive(Path.Join(_originRoaming, "Logs"), $"{localCachePath}Logs");
            GeneralFuncs.CopyFilesRecursive(Path.Join(_originProgramData, "Subscription"), $"{localCachePathData}Subscription");

            var pfpFilePath = "";
            foreach (var f in new DirectoryInfo(_originRoaming).GetFiles())
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

            Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\"));
            File.Copy((pfpFilePath != "" ? pfpFilePath :  Path.Join(GeneralFuncs.WwwRoot,"img\\QuestionMark.jpg"))!, Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\{Uri.EscapeUriString(accName)}.jpg"), true);

            var olcHashes = new List<string>();
            foreach (var f in new DirectoryInfo(_originProgramData).GetFiles())
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
            TValue value;
            if (!dict.Remove(oldKey, out value))
                return false;

            dict[newKey] = value;
            return true;
        }

        private static Dictionary<string, List<string>> ReadAllOlc()
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.ReadAllOlc]");
            var localAllOlc = $"LoginCache\\Origin\\olc.json";
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
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.CloseSteam]");
            if (!GeneralFuncs.CanKillProcess("origin")) return false;
            Globals.KillProcess("origin");
            return true;
        }
        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        /// <returns>Whether files were deleted or not</returns>
        public static async void ClearImages()
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.ClearImages] Clearing images.");
            //if (!Directory.Exists(Steam.SteamImagePath))
            //{
            //    await GeneralInvocableFuncs.ShowToast("error", "Could not clear images", "Error", "toastarea");
            //}
            //foreach (var file in Directory.GetFiles(Origin.OriginImage))
            //{
            //    File.Delete(file);
            //}
            //// Reload page, then display notification using a new thread.
            AppData.ActiveNavMan?.NavigateTo("/Origin/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared images"), true);
        }

        /// <summary>
        /// Only runs ForgetAccount, but allows Javascript to wait for it's completion before refreshing, instead of just doing it instantly >> Not showing proper results.
        /// </summary>
        /// <param name="accName">Origin account name to be removed from cache</param>
        /// <returns>true</returns>
        [JSInvokable]
        public static Task<bool> ForgetOriginAccountJs(string accName)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Origin\OriginSwitcherFuncs.ForgetOriginAccountJs] accName:{accName}");
            var allOlc = ReadAllOlc();
            allOlc.Remove(accName);
            var olcHashString = JsonConvert.SerializeObject(allOlc);
            File.WriteAllText("LoginCache\\Origin\\olc.json", olcHashString);
            return Task.FromResult(ForgetAccount(accName));
        }

        #endregion
    }
}
