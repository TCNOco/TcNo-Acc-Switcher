// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using DiscordRPC;
using DiscordRPC.Logging;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using Index = TcNo_Acc_Switcher_Server.Pages.Steam.Index;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppData
    {
        private static readonly Lang Lang = Lang.Instance;
        private static AppData _instance = new();

        private static readonly object LockObj = new();

        public static AppData Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new AppData();
                }
            }
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }

        #region First Launch
        // I'm not really sure where to include this for the first page visit, so it is here and called on every page.
        // As far as I understand this works like a browser, and getting it to try and display something may not work the best...
        // So it's called on each page, but only run once.
        private bool _firstLaunch = true;
        private static bool FirstLaunch { get => Instance._firstLaunch; set => Instance._firstLaunch = value; }
        /// <summary>
        /// (Only one time) Checks for update, and submits statistics if enabled.
        /// </summary>
        public static void FirstLaunchCheck()
        {
            if (!FirstLaunch) return;
            FirstLaunch = false;
            // Check for update in another thread
            // Also submit statistics, if enabled
            new Thread(AppSettings.CheckForUpdate).Start();
            if (AppSettings.StatsEnabled && AppSettings.StatsShare)
                new Thread(AppStats.UploadStats).Start();

            // Discord integration
            RefreshDiscordPresenceAsync();

            // Unused. Idk if I will use these, but they are here just in-case.
            //DiscordClient.OnReady += (sender, e) => { Console.WriteLine("Received Ready from user {0}", e.User.Username); };
            //DiscordClient.OnPresenceUpdate += (sender, e) => { Console.WriteLine("Received Update! {0}", e.Presence); };
        }

        public static void RefreshDiscordPresenceAsync()
        {
            var dThread = new Thread(RefreshDiscordPresence);
            dThread.Start();
        }

        public static void RefreshDiscordPresence()
        {
            Thread.Sleep(1000);

            if (!AppSettings.DiscordRpc)
            {
                if (DiscordClient != null)
                {
                    if (!DiscordClient.IsInitialized) return;
                    DiscordClient.Deinitialize();
                    DiscordClient = null;
                }

                return;
            }

            var timestamp = Timestamps.Now;

            DiscordClient ??= new DiscordRpcClient("973188269405765682")
            {
                Logger = new ConsoleLogger { Level = LogLevel.Warning },
            };
            if (!DiscordClient.IsInitialized) DiscordClient.Initialize();
            else timestamp = DiscordClient.CurrentPresence.Timestamps;

            var state = "";
            if (AppSettings.StatsEnabled && AppSettings.DiscordRpcShare)
            {
                AppStats.GenerateTotals();
                state = Lang["Discord_StatusDetails", new { number = AppStats.SwitcherStats["_Total"].Switches }];
            }


            DiscordClient.SetPresence(new RichPresence
            {
                Details = Lang["Discord_Status"],
                State = state,
                Timestamps = timestamp,
                Buttons = new Button[]
                { new() {
                    Url = "https://github.com/TcNobo/TcNo-Acc-Switcher/",
                    Label = Lang["Website"]
                }},
                Assets = new Assets
                {
                    LargeImageKey = "switcher",
                    LargeImageText = "TcNo Account Switcher"
                }
            });
        }
        #endregion

        #region Accounts
        // These hold accounts for different switchers. Changing settings reloads the Settings, wiping them. This is the second best place to store these.

        private List<Index.Steamuser> _steamUsers;
        public static List<Index.Steamuser> SteamUsers { get => Instance._steamUsers; set => Instance._steamUsers = value; }

        private bool _steamLoadingProfiles;
        public static bool SteamLoadingProfiles { get => Instance._steamLoadingProfiles; set => Instance._steamLoadingProfiles = value; }
        #endregion

        // Window stuff
        private string _windowTitle = "TcNo Account Switcher";

        public static string WindowTitle
        {
            get => Instance._windowTitle;
            set
            {
                Instance._windowTitle = value;
                Instance.NotifyDataChanged();
                Globals.WriteToLog($"{Environment.NewLine}Window Title changed to: {value}");
            }
        }

        private string _currentStatus = Lang["Status_Init"];
        public static string CurrentStatus
        {
            get => Instance._currentStatus;
            set => Instance._currentStatus = value;
        }

        public static string SelectedAccountId => SelectedAccount.AccountId;

        private Account _selectedAccount;
        public static Account SelectedAccount
        {
            get => Instance._selectedAccount;
            set => Instance._selectedAccount = value;
        }

        private string _currentSwitcher = "";
        public static string CurrentSwitcher
        {
            get => Instance._currentSwitcher;
            set
            {
                Instance._currentSwitcher = value;
                CurrentSwitcherSafe = Globals.GetCleanFilePath(CurrentSwitcher);
            }
        }

        private string _currentSwitcherSafe = "";
        public static string CurrentSwitcherSafe { get => Instance._currentSwitcherSafe; set => Instance._currentSwitcherSafe = value; }

        #region Basic_Platforms

        public List<string> OldPlatformList = new() { "Steam" };
        private List<string> _platformList;
        public List<string> PlatformList
        {
            get
            {
                Instance._platformList = new List<string>(OldPlatformList);

                // Add enabled basic platforms:
                Instance._platformList =
                    Instance._platformList.Union(AppSettings.EnabledBasicPlatforms).ToList();
                return Instance._platformList;
            }
            set => Instance._platformList = value;
        }

        public List<string> SortedPlatformListHandleDisabled() => GenericFunctions.OrderAccounts(
            Instance.PlatformList.Where(p => !AppSettings.DisabledPlatforms.Contains(p)).ToList(),
            "Settings\\platformOrder.json");

        public List<string> EnabledPlatformSorted()
        {
            var enabled = Instance.PlatformList.Where(p => !AppSettings.DisabledPlatforms.Contains(p)).ToList();
            enabled.Sort(StringComparer.InvariantCultureIgnoreCase);
            return enabled;
        }
        public List<string> DisabledPlatformSorted()
        {
            var disabled = new List<string>(BasicPlatforms.InactivePlatforms().Keys);
            disabled = disabled.Concat(Instance.PlatformList.Where(p => AppSettings.DisabledPlatforms.Contains(p)).ToList()).ToList();
            disabled.Sort(StringComparer.InvariantCultureIgnoreCase);
            return disabled;
        }

        public bool AnyPlatformsShowing() => Instance.PlatformList.Count > AppSettings.DisabledPlatforms.Count;
        #endregion

        public event Action OnChange;

        public void NotifyDataChanged() => OnChange?.Invoke();

        private IJSRuntime _activeIJsRuntime;
        [JsonIgnore] public static IJSRuntime ActiveIJsRuntime { get => Instance._activeIJsRuntime; set => Instance._activeIJsRuntime = value; }
        public void SetActiveIJsRuntime(IJSRuntime jsr) => Instance._activeIJsRuntime = jsr;

        private NavigationManager _activeNavMan;
        [JsonIgnore] public static NavigationManager ActiveNavMan { get => Instance._activeNavMan; set => Instance._activeNavMan = value; }

        private bool _firstMainMenuVisit = true;
        [JsonIgnore] public static bool FirstMainMenuVisit { get => Instance._firstMainMenuVisit; set => Instance._firstMainMenuVisit = value; }
        public void SetActiveNavMan(NavigationManager nm) => Instance._activeNavMan = nm;


        private InitializedClasses _initializedClasses = new();
        [JsonIgnore] public static InitializedClasses InitializedClasses { get => Instance._initializedClasses; set => Instance._initializedClasses = value; }


        private DiscordRpcClient _discordClient;
        [JsonIgnore] public static DiscordRpcClient DiscordClient { get => Instance._discordClient; set => Instance._discordClient = value; }

        /// <summary>
        /// Contains whether the Client app is running, and the server is it's child.
        /// </summary>
        private bool _tcNoClientApp;
        [JsonIgnore] public static bool TcNoClientApp { get => Instance._tcNoClientApp; set => Instance._tcNoClientApp = value; }

        #region JS_INTEROP
        /// <summary>
        /// A general wrapper for InvokeVoidAsync, that returns true if it ran, or false if it doesn't.
        /// </summary>
        public static async Task<bool> InvokeVoidAsync(string func, params object?[]? args)
        {
            Globals.WriteToLog(!Globals.VerboseMode ? $"JS InvokeVoidAsync: {func}" : $"JS CALL: {func}: {args}");
            try
            {
                if (ActiveIJsRuntime is null) return false;
                await ActiveIJsRuntime.InvokeVoidAsync(func, args);
                return true;
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return false;
            }
        }

        /// <summary>
        /// A general wrapper for InvokeVoid. This will do nothing if ActiveIJsRuntime is null.
        /// </summary>
        public static async Task<TValue> InvokeAsync<TValue>(string func, params object?[]? args)
        {
            Globals.WriteToLog(!Globals.VerboseMode ? $"JS InvokeAsync: {func}" : $"JS CALL: {func}: {args}");
            try
            {
                if (ActiveIJsRuntime is null) return default;
                return await ActiveIJsRuntime.InvokeAsync<TValue>(func, args);
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return default;
            }
        }

        public static async Task ReloadPage() => await ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        public static async Task CacheReloadPage() => await ActiveIJsRuntime.InvokeVoidAsync("location.reload(true);");
        #endregion


        [JsonIgnore] private ObservableCollection<Account> _steamAccounts = new();
        public static ObservableCollection<Account> SteamAccounts
        {
            get => Instance._steamAccounts;
            set => Instance._steamAccounts = value;
        }

        [JsonIgnore] private ObservableCollection<Account> _basicAccounts = new();
        public static ObservableCollection<Account> BasicAccounts { get => Instance._basicAccounts; set => Instance._basicAccounts = value; }
    }
    public class InitializedClasses
    {
        public InitializedClasses()
        {
            Basic = false;
            Steam = false;
        }
        public bool Basic { get; set; }
        public bool Steam { get; set; }
    }
}
