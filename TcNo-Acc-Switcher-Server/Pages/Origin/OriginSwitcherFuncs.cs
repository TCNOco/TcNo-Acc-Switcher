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
        static string OriginRoaming;
        static string OriginProgramData;
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
            OriginRoaming = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Origin");
            OriginProgramData = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Origin");


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
                Debug.WriteLine(f);
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
        public static void SwapOriginAccounts(string accName = "", int state = 0)
        {
            Globals.DebugWriteLine($@"[Func:Origin\OriginSwitcherFuncs.SwapOriginAccounts] Swapping to: {accName}.");
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Closing Origin");
            CloseOrigin();
            // DO ACTUAL SWITCHING HERE
            ClearCurrentLoginOrigin();
            if (accName != "")
            {
                OriginCopyInAccount(accName, state);
            }
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting Origin");
            if (Origin.Admin)
                Process.Start(Origin.Exe());
            else
                Process.Start(new ProcessStartInfo("explorer.exe", Origin.Exe()));
        }


        private static void ClearCurrentLoginOrigin()
        {
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

            Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\"));
            File.Copy((pfpFilePath != "" ? pfpFilePath :  Path.Join(GeneralFuncs.WwwRoot,"img\\QuestionMark.jpg"))!, Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\origin\\{Uri.EscapeUriString(accName)}.jpg"), true);

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

            var olcHashString = JsonConvert.SerializeObject(allOlc);
            File.WriteAllText("LoginCache\\Origin\\olc.json", olcHashString);
            
            AppData.ActiveNavMan?.NavigateTo("/Origin/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + accName), true);
        }

        private static Dictionary<string, List<string>> ReadAllOlc()
        {
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
        public static void CloseOrigin()
        {
            Globals.KillProcess("origin");
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
