using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using State;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Classes.Templated;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State
{
    public class TemplatedPlatformState
    {
        [Inject] private SharedFunctions SharedFunctions { get; set; }
        [Inject] private JSRuntime JsRuntime { get; set; }
        [Inject] private IWindowSettings WindowSettings { get; set; }
        [Inject] private IStatistics Statistics { get; set; }
        [Inject] private Modals Modals { get; set; }
        [Inject] private IAppState AppState { get; set; }
        [Inject] private Toasts Toasts { get; set; }

        public List<string> AvailablePlatforms { get; set; }

        public Platform CurrentPlatform { get; set; }
        public List<Platform> Platforms { get; set; }

        // Replace the other PlatformItem.cs here
        // Just have this load all the names and identifiers as well.

        private readonly string _platformJsonPath = Path.Join(Globals.AppDataFolder, "Platforms.json");
        public TemplatedPlatformState()
        {
            if (!File.Exists(_platformJsonPath))
            {
                Toasts.ShowToastLang(ToastType.Error, "Toast_FailedPlatformsLoad");
                Globals.WriteToLog("Failed to locate Platforms.json! This will cause a lot of features to break.");
                return;
            }

            Platforms = JsonConvert.DeserializeObject<List<Platform>>(_platformJsonPath);
            if (Platforms is null) return;
            AvailablePlatforms = Platforms.Select(x => x.Name).ToList();

            // Import list to master platform list -- For displaying on the main menu.
            foreach (var plat in Platforms)
            {
                // Add to master platform list
                if (WindowSettings.Platforms.Count(y => y.Name == plat.Name) == 0)
                    WindowSettings.Platforms.Add(new PlatformItem(plat.Name, plat.Identifiers, plat.ExeName, false));
                else
                {
                    // Make sure that everything is set up properly.
                    var platform = WindowSettings.Platforms.First(y => y.Name == plat.Name);
                    platform.SetFromPlatformItem(new PlatformItem(plat.Name, plat.Identifiers, plat.ExeName, false));
                }
            }

        }

        public void SetCurrentPlatform(string platformName)
        {
            CurrentPlatform = Platforms.First(x => x.Name == platformName || x.Identifiers.Contains(platformName));
            CurrentPlatform.InitAfterDeserialization();
            CurrentPlatform.PlatformSavedSettings = new PlatformSavedSettings(); // Load saved settings

            AppState.Switcher.CurrentSwitcher = CurrentPlatform.Name;
            AppState.Switcher.TemplatedAccounts.Clear();
            LoadAccounts();
        }

        #region Loading
        private async Task<bool> LoadAccounts()
        {
            var localCachePath = Path.Join(Globals.UserDataFolder, $"LoginCache\\{CurrentPlatform.SafeName}\\");
            if (!Directory.Exists(localCachePath)) return false;
            if (!ListAccountsFromFolder(localCachePath, out var accList)) return false;

            // Order
            accList = OrderAccounts(accList, $"{localCachePath}\\order.json");

            await InsertAccounts(accList);
            Statistics.SetAccountCount(CurrentPlatform.SafeName, accList.Count);

            // Load notes
            LoadNotes();
            return true;
        }

        /// <summary>
        /// Gets a list of 'account names' from cache folder provided
        /// </summary>
        /// <param name="folder">Cache folder containing accounts</param>
        /// <param name="accList">List of account strings</param>
        /// <returns>Whether the directory exists and successfully added listed names</returns>
        private bool ListAccountsFromFolder(string folder, out List<string> accList)
        {
            accList = new List<string>();

            if (!Directory.Exists(folder)) return false;
            var idsFile = Path.Join(folder, "ids.json");
            accList = File.Exists(idsFile)
                ? Globals.ReadDict(idsFile).Keys.ToList()
                : (from f in Directory.GetDirectories(folder)
                   where !f.EndsWith("Shortcuts")
                   let lastSlash = f.LastIndexOf("\\", StringComparison.Ordinal) + 1
                   select f[lastSlash..]).ToList();

            return true;
        }

        /// <summary>
        /// Orders a list of strings by order specific in jsonOrderFile
        /// </summary>
        /// <param name="accList">List of strings to sort</param>
        /// <param name="jsonOrderFile">JSON file containing list order</param>
        /// <returns></returns>
        private List<string> OrderAccounts(List<string> accList, string jsonOrderFile)
        {
            // Order
            if (!File.Exists(jsonOrderFile)) return accList;
            var savedOrder = JsonConvert.DeserializeObject<List<string>>(Globals.ReadAllText(jsonOrderFile));
            if (savedOrder == null) return accList;
            var index = 0;
            if (savedOrder is not { Count: > 0 }) return accList;
            foreach (var acc in from i in savedOrder where accList.Any(x => x == i) select accList.Single(x => x == i))
            {
                _ = accList.Remove(acc);
                accList.Insert(Math.Min(index, accList.Count), acc);
                index++;
            }
            return accList;
        }

        private void LoadNotes()
        {
            var filePath = $"LoginCache\\{CurrentPlatform.SafeName}\\AccountNotes.json";
            if (!File.Exists(filePath)) return;

            var loaded = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(filePath));
            if (loaded is null) return;

            foreach (var (key, val) in loaded)
            {
                var acc = AppState.Switcher.TemplatedAccounts.FirstOrDefault(x => x.AccountId == key);
                if (acc is null) return;
                acc.Note = val;
            }

            Modals.IsShown = false;
        }


        /// <summary>
        /// Iterate through account list and insert into platforms account screen
        /// </summary>
        /// <param name="accList">Account list</param>
        private async Task InsertAccounts(List<string> accList)
        {
            LoadAccountIds();

            AppState.Switcher.TemplatedAccounts.Clear();

            foreach (var str in accList)
            {
                var account = new Account
                {
                    Platform = CurrentPlatform.SafeName,
                    AccountId = str,
                    DisplayName = GetNameFromId(str),
                    // Handle account image
                    ImagePath = GetImgPath(CurrentPlatform.SafeName, str).Replace("%", "%25")
                };

                var actualImagePath = Path.Join("wwwroot\\", GetImgPath(CurrentPlatform.SafeName, str));
                if (!File.Exists(actualImagePath))
                {
                    // Make sure the directory exists
                    Directory.CreateDirectory(Path.GetDirectoryName(actualImagePath)!);
                    var defaultPng = $"wwwroot\\img\\platform\\{CurrentPlatform.SafeName}Default.png";
                    const string defaultFallback = "wwwroot\\img\\BasicDefault.png";
                    if (File.Exists(defaultPng))
                        Globals.CopyFile(defaultPng, actualImagePath);
                    else if (File.Exists(defaultFallback))
                        Globals.CopyFile(defaultFallback, actualImagePath);
                }

                // Handle game stats (if any enabled and collected.)
                account.UserStats = GetUserStatsAllGamesMarkup(str);

                AppState.Switcher.TemplatedAccounts.Add(account);
            }
            await SharedFunctions.FinaliseAccountList(); // Init context menu & Sorting
        }

        /// <summary>
        /// Finds if file exists as .jpg or .png
        /// </summary>
        /// <param name="platform">Platform name</param>
        /// <param name="user">Username/ID for use in image name</param>
        /// <returns>Image path</returns>
        private static string GetImgPath(string platform, string user)
        {
            var imgPath = $"\\img\\profiles\\{platform.ToLowerInvariant()}\\{Globals.GetCleanFilePath(user.Replace("#", "-"))}";
            if (File.Exists("wwwroot\\" + imgPath + ".png")) return imgPath + ".png";
            return imgPath + ".jpg";
        }

        #endregion


        #region Account IDs
        public Dictionary<string, string> AccountIds;
        public void LoadAccountIds() => AccountIds = Globals.ReadDict(CurrentPlatform.IdsJsonPath);
        private void SaveAccountIds() =>
            File.WriteAllText(CurrentPlatform.IdsJsonPath, JsonConvert.SerializeObject(AccountIds));
        public string GetNameFromId(string accId) => AccountIds.ContainsKey(accId) ? AccountIds[accId] : accId;
        #endregion

        public void RunPlatform(bool admin, string args = "")
        {
            if (Globals.StartProgram(CurrentPlatform.PlatformSavedSettings.Exe, admin, args, CurrentPlatform.Extras.StartingMethod))
                Toasts.ShowToastLang(ToastType.Info, new LangSub("Status_StartingPlatform", new { platform = CurrentPlatform.Name }));
            else
                Toasts.ShowToastLang(ToastType.Error, new LangSub("Toast_StartingPlatformFailed", new { platform = CurrentPlatform.Name }));
        }

        // This seemed to not be used, so I have omitted it here.
        // public void RunPlatform(bool admin)
        public void RunPlatform() => RunPlatform(CurrentPlatform.PlatformSavedSettings.Admin, CurrentPlatform.ExeExtraArgs);

        public void HandleShortcutAction(string shortcut, string action)
        {
            if (shortcut == "btnStartPlat") // Start platform requested
            {
                RunPlatform(action == "admin");
                return;
            }

            if (!CurrentPlatform.PlatformSavedSettings.Shortcuts.ContainsValue(shortcut)) return;

            switch (action)
            {
                case "hide":
                {
                    // Remove shortcut from folder, and list.
                    CurrentPlatform.PlatformSavedSettings.Shortcuts.Remove(CurrentPlatform.PlatformSavedSettings.Shortcuts.First(e => e.Value == shortcut).Key);
                    var f = Path.Join(CurrentPlatform.ShortcutFolder, shortcut);
                    if (File.Exists(f)) File.Move(f, f.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));

                        // Save.
                        CurrentPlatform.PlatformSavedSettings.Save();
                    break;
                }
                case "admin":
                    SharedFunctions.RunShortcut(shortcut, CurrentPlatform.ShortcutFolder, admin: true);
                    break;
            }
        }
    }
}
