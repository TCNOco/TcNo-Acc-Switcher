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
        private static readonly Lang Lang = Lang.Instance;

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
            Data.Settings.Riot.Instance.LoadFromFile();

            LoadImportantData(); // If not already loaded -- Likely it will be already
            if (DelayedToasts.Count > 0)
            {
                foreach (var delayedToast in DelayedToasts)
                {
                    _ = GeneralInvocableFuncs.ShowToast(delayedToast[0], delayedToast[1], delayedToast[2], "toastarea");
                }
            }

            _ = GenericFunctions.GenericLoadAccounts("Riot");
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

            var o = JObject.Parse(Globals.ReadAllText(riotClientInstallsFile));
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
        /// Arguments set by CLI, for use when starting Riot games.
        /// </summary>
        private static string _startArguments = "";

        /// <summary>
        /// Restart Riot with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        /// <param name="args">Starting arguments</param>
        public static void SwapRiotAccounts(string accName, string args = "")
        {
            Globals.DebugWriteLine(@"[Func:Riot\RiotSwitcherFuncs.SwapRiotAccounts] Swapping to: hidden.");
            _startArguments = " " + args;


            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatform", new { platform = "Riot" }]);
            if (!GeneralFuncs.CloseProcesses(Data.Settings.Riot.Processes, Data.Settings.Riot.Instance.AltClose))
            {
                _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = "Riot" }]);
                return;
            };

            ClearCurrentLoginRiot();
            if (accName != "")
            {
                if (!RiotCopyInAccount(accName)) return;
                Globals.AddTrayUser("Riot", "+r:" + accName, accName, Riot.TrayAccNumber); // Add to Tray list
                _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_Riot_StartGame"], Lang["Success"], "toastarea");
            }

            //GeneralFuncs.StartProgram(Riot.Exe(), Riot.Admin);

            Globals.RefreshTrayArea();
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
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
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotFindX", new { x = localCachePath }], Lang["DirectoryNotFound"], "toastarea");
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
            _ = Directory.CreateDirectory(localCachePath);

            if (!File.Exists(_riotClientPrivateSettings) || !File.Exists(_riotClientConfig))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_CouldNotLocate"], Lang["Failed"], "toastarea");
                return;
            }
            // Save files
            File.Copy(_riotClientPrivateSettings, Path.Join(localCachePath, "RiotClientPrivateSettings.yaml"), true);
            File.Copy(_riotClientConfig, Path.Join(localCachePath, "RiotClientSettings.yaml"), true);

            // Copy in profile image from default
            _ = Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot(), "\\img\\profiles\\riot"));
            File.Copy(Path.Join(GeneralFuncs.WwwRoot(), "\\img\\RiotDefault.png"), Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\riot\\{accName.Replace("#", "-")}.jpg"), true);

            AppData.ActiveNavMan?.NavigateTo("/Riot/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_SavedItem", new { item = accName }]), true);
        }

        public static void ChangeUsername(string oldName, string newName, bool reload = true)
        {
            File.Move(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\riot\\{Globals.GetCleanFilePath(oldName).Replace("#", "-")}.jpg"),
                Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\riot\\{Globals.GetCleanFilePath(newName).Replace("#", "-")}.jpg")); // Rename image
            Directory.Move($"LoginCache\\Riot\\{oldName}\\", $"LoginCache\\Riot\\{newName}\\"); // Rename login cache folder

            if (reload) AppData.ActiveNavMan?.NavigateTo("/Riot/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_ChangedUsername"]), true);
        }

        #region RIOT_MANAGEMENT
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
                    args = "--launch-product=league_of_legends --launch-patchline=live" + _startArguments;
                    name = "League of Legends";
                    break;
                case 'r':
                    dir = Riot.RuneterraRiotDir;
                    args = "--launch-product=bacon --launch-patchline=live" + _startArguments;
                    name = "Legends of Runeterra";
                    break;
                case 'v':
                    dir = Riot.ValorantRiotDir;
                    args = "--launch-product=valorant --launch-patchline=live" + _startArguments;
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
            _ = proc.Start();

            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_StartedGame", new { program = name }], Lang["Success"], "toastarea");
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

            _ = Process.Start("explorer.exe", dir.Replace("/", "\\"));
        }
        #endregion
    }
}
