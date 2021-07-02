using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Riot
{
    public class RiotSwitcherFuncs
    {
        private static readonly Data.Settings.Riot Riot = Data.Settings.Riot.Instance;
        private static string _riotClientPrivateSettings = "",
	        _riotClientConfig = "";

        /// <summary>
        /// Main function for Riot Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static void LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine(@"[Func:Riot\RiotSwitcherFuncs.LoadProfiles] Loading Riot profiles");

            LoadImportantData(); // If not already loaded -- Likely it will be already
            if (DelayedToasts.Count > 0)
            {
                foreach (var delayedToast in DelayedToasts)
                {
                    _ = GeneralInvocableFuncs.ShowToast(delayedToast[0], delayedToast[1], delayedToast[2], "toastarea");
                }
            }

            GenericFunctions.GenericLoadAccounts("Riot");
        }

        // Delayed toasts, as notifications are created in the LoadImportantData() section, and can be before the main process has rendered items.
        private static readonly List<List<string>> DelayedToasts = new();

        /// <summary>
        /// Run necessary functions and load data when being launcher without a GUI (From command line for example).
        /// </summary>
        public static void LoadImportantData()
        {
            // Load once
            if (Riot.Initialised) return;

            _riotClientPrivateSettings = Path.Join(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Riot Games\\Riot Client\\Data", "RiotClientPrivateSettings.yaml"));
            _riotClientConfig = Path.Join(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Riot Games\\Riot Client\\Config", "RiotClientSettings.yaml"));

            // Check what games are installed:
            var riotClientInstallsFile = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Riot Games\\RiotClientInstalls.json");
            if (!File.Exists(riotClientInstallsFile)) return;

            var o = JObject.Parse(File.ReadAllText(riotClientInstallsFile));
            if (!o.ContainsKey("associated_client")) return;

            var assocClient = (JObject)o["associated_client"];
            if (assocClient == null) return;
            Riot.LeagueDir = null;
            Riot.RuneterraDir = null;
            Riot.ValorantDir = null;
            foreach (var (key, value) in assocClient)
            {
                if (key.Contains("League"))
                {
                    if (Riot.LeagueDir != null)
                    {
                        DelayedToasts.Add(new List<string> { "error", "More than 1 League install found", "Duplicate Install" });
                        continue;
                    }
                    Riot.LeagueDir = key.Replace('/', '\\');
                    Riot.LeagueRiotDir = ((string)value)?.Replace('/', '\\');
                }
                else if (key.Contains("LoR"))
                {
                    if (Riot.RuneterraDir != null)
                    {
                        DelayedToasts.Add(new List<string> { "error", "More than 1 Runeterra install found", "Duplicate Install" });
                        continue;
                    }
                    Riot.RuneterraDir = key.Replace('/', '\\');
                    Riot.RuneterraRiotDir = ((string)value)?.Replace('/', '\\');
                }
                else if (key.Contains("VALORANT"))
                {
                    if (Riot.ValorantDir != null)
                    {
                        DelayedToasts.Add(new List<string> { "error", "More than 1 VALORANT install found", "Duplicate Install" });
                        continue;
                    }
                    Riot.ValorantDir = key.Replace('/', '\\');
                    Riot.ValorantRiotDir = ((string)value)?.Replace('/', '\\');
                }
            }

            Riot.Initialised = true;
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetRiotForgetAcc() => Task.FromResult(Riot.ForgetAccountEnabled);

        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="accName">Riot account name</param>
        public static bool ForgetAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:RiotRiotSwitcherFuncs.ForgetAccount] Forgetting account: hidden");
            // Remove image
            var img = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\riot\\{accName.Replace("#", "-")}.jpg");
            if (File.Exists(img)) File.Delete(img);
            // Remove cached files
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Riot\\{accName}"), false);
            // Remove from Tray
            Globals.RemoveTrayUser("Riot", accName); // Add to Tray list
            return true;
        }
        
        /// <summary>
        /// Restart Riot with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        public static void SwapRiotAccounts(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Riot\RiotSwitcherFuncs.SwapRiotAccounts] Swapping to: hidden.");
            AppData.InvokeVoidAsync("updateStatus", "Closing Riot");
            if (!CloseRiot()) return;
            ClearCurrentLoginRiot();
            if (accName != "")
            {
                if (!RiotCopyInAccount(accName)) return;
                Globals.AddTrayUser("Riot", "+r:" + accName, accName, Riot.TrayAccNumber); // Add to Tray list
                _ = GeneralInvocableFuncs.ShowToast("success", "Changed user. Start a game below.", "Success", "toastarea");
            }

            //GeneralFuncs.StartProgram(Riot.Exe(), Riot.Admin);

            AppData.InvokeVoidAsync("updateStatus", "Ready");
            Globals.RefreshTrayArea();
        }
        
        private static void ClearCurrentLoginRiot()
        {
            Globals.DebugWriteLine(@"[Func:Riot\RiotSwitcherFuncs.ClearCurrentLoginRiot]");
            if (File.Exists(_riotClientPrivateSettings)) File.Delete(_riotClientPrivateSettings);
            if (File.Exists(_riotClientConfig)) File.Delete(_riotClientConfig);
        }

        private static bool RiotCopyInAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Riot\RiotSwitcherFuncs.RiotCopyInAccount]");
            LoadImportantData();
            var localCachePath = $"LoginCache\\Riot\\{accName}\\";
            if (!Directory.Exists(localCachePath))
            {
	            _ = GeneralInvocableFuncs.ShowToast("error", $"Could not find {localCachePath}", "Directory not found", "toastarea");
	            return false;
            }

            File.Copy($"{localCachePath}RiotClientPrivateSettings.yaml", _riotClientPrivateSettings, true);
            File.Copy($"{localCachePath}RiotClientSettings.yaml", _riotClientConfig, true);
            return true;
        }
        
        public static void RiotAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Riot\RiotSwitcherFuncs.RiotAddCurrent]");
            var localCachePath = $"LoginCache\\Riot\\{accName}\\";
            Directory.CreateDirectory(localCachePath);

            if (!File.Exists(_riotClientPrivateSettings) || !File.Exists(_riotClientConfig))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not locate logged in user", "Failed", "toastarea");
                return;
            }
            // Save files
            File.Copy(_riotClientPrivateSettings, Path.Join(localCachePath, "RiotClientPrivateSettings.yaml"), true);
            File.Copy(_riotClientConfig, Path.Join(localCachePath, "RiotClientSettings.yaml"), true);
            
            // Copy in profile image from default
            Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot(), "\\img\\profiles\\riot"));
            File.Copy(Path.Join(GeneralFuncs.WwwRoot(), "\\img\\RiotDefault.png"), Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\riot\\{accName.Replace("#", "-")}.jpg"), true);

            AppData.ActiveNavMan?.NavigateTo("/Riot/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + accName), true);
        }

        public static void ChangeUsername(string oldName, string newName, bool reload = true)
        {
            File.Move(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\riot\\{Uri.EscapeUriString(oldName).Replace("#", "-")}.jpg"),
                Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\riot\\{Uri.EscapeUriString(newName).Replace("#", "-")}.jpg")); // Rename image
            Directory.Move($"LoginCache\\Riot\\{oldName}\\", $"LoginCache\\Riot\\{newName}\\"); // Rename login cache folder

            if (reload) AppData.ActiveNavMan?.NavigateTo("/Riot/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Changed username"), true);
        }

        #region RIOT_MANAGEMENT
        /// <summary>
        /// List of Riot processes to close
        /// </summary>
        private static readonly string[] RiotProcessList = { "LeagueClient.exe", "LoR.exe", "VALORANT.exe", "RiotClientServices.exe", "RiotClientUx.exe", "RiotClientUxRender.exe" };
        
        /// <summary>
        /// Returns true if program can kill all Riot processes
        /// </summary>
        public static bool CanCloseRiot() => RiotProcessList.Aggregate(true, (current, s) => current & GeneralFuncs.CanKillProcess(s));

        /// <summary>
        /// Kills Riot processes when run via cmd.exe
        /// </summary>
        public static bool CloseRiot()
        {
            Globals.DebugWriteLine(@"[Func:Riot\RiotSwitcherFuncs.CloseRiot]");
            if (!CanCloseRiot()) return false;

            // Kill game clients & Platform clients
            foreach (var s in RiotProcessList)
            {
                Globals.KillProcess(s);
                GeneralFuncs.WaitForClose(s);
            }
            return true;
        }

        /// <summary>
        /// Start Riot games
        /// </summary>
        /// <param name="game"></param>
        public static void RiotStart(char game)
        {
            string name = "", dir = "", args = "";
            switch (game)
            {
                case 'l':
                    dir = Riot.LeagueRiotDir;
                    args = "--launch-product=league_of_legends --launch-patchline=live";
                    name = "League of Legends";
                    break;
                case 'r':
                    dir = Riot.RuneterraRiotDir;
                    args = "--launch-product=bacon --launch-patchline=live";
                    name = "Legends of Runeterra";
                    break;
                case 'v':
                    dir = Riot.ValorantRiotDir;
                    args = "--launch-product=valorant --launch-patchline=live";
                    name = "Valorant";
                    break;
            }

            var proc = new Process
            {
                StartInfo =
                {
                    FileName = dir,
                    Arguments = args,
                    UseShellExecute = true,
                    Verb = Riot.Admin ? "runas" : ""
                }
            };
            proc.Start();

            _ = GeneralInvocableFuncs.ShowToast("info", "Started " + name, "Success", "toastarea");
        }

        /// <summary>
        /// Open Riot Games game folders
        /// </summary>
        /// <param name="game"></param>
        public static void RiotOpenFolder(char game)
        {
            var dir = game switch
            {
                'l' => Riot.LeagueDir,
                'r' => Riot.RuneterraDir,
                'v' => Riot.ValorantDir,
                _ => ""
            };

            Process.Start("explorer.exe", dir.Replace("/", "\\"));
        }
        #endregion
    }
}
