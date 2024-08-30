// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using DiscordRPC;
using DiscordRPC.Logging;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
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

        public static void OfflineMode_Toggle()
        {
            RefreshDiscordPresenceAsync();
        }

        public static void RefreshDiscordPresenceAsync()
        {
            var dThread = new Thread(RefreshDiscordPresence);
            dThread.Start();
        }

        public static void RefreshDiscordPresence()
        {
            Thread.Sleep(1000);

            if (AppSettings.OfflineMode || !AppSettings.DiscordRpc)
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

        public string WindowTitle
        {
            get => _windowTitle;
            set
            {
                _windowTitle = value;
                NotifyDataChanged();
                Globals.WriteToLog($"{Environment.NewLine}Window Title changed to: {_windowTitle}");
            }
        }

        private string _currentStatus = Lang["Status_Init"];
        public string CurrentStatus
        {
            get => _currentStatus;
            set
            {
                _currentStatus = value;
                NotifyDataChanged();
            }
        }

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
        
        // Check if any items in the PlatformList are not hidden
        public bool AnyPlatformsShowing() => !Instance.PlatformList.All(item => AppSettings.DisabledPlatforms.Contains(item));
        #endregion

        public event Action OnChange;

        private void NotifyDataChanged() => OnChange?.Invoke();

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

        private bool _updatePending = false;
        [JsonIgnore] public static bool UpdatePending { get => Instance._updatePending; set => Instance._updatePending = value; }

        #region JS_INTEROP
        public static bool InvokeVoidAsync(string func)
        {
            Globals.WriteToLog($"JS CALL (1): {func}");
            try
            {
                if (ActiveIJsRuntime is null) return false;
                ActiveIJsRuntime.InvokeVoidAsync(func);
                return true;
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return false;
            }
        }

        public static bool InvokeVoidAsync(string func, string arg, bool showDetails = true)
        {
            Globals.WriteToLog(!showDetails && !Globals.VerboseMode ? $"JS CALL (2): {func}..." : $"JS CALL (4): {func}, {arg}");
            try
            {
                if (ActiveIJsRuntime is null) return false;
                ActiveIJsRuntime.InvokeVoidAsync(func, arg);
                return true;
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return false;
            }
        }

        public static bool InvokeVoidAsync(string func, object arg, bool showDetails = true)
        {
            Globals.WriteToLog(!showDetails && !Globals.VerboseMode ? $"JS CALL (3): {func}..." : $"JS CALL (4): {func}, {JsonConvert.SerializeObject(arg)}");
            try
            {
                if (ActiveIJsRuntime is null) return false;
                ActiveIJsRuntime.InvokeVoidAsync(func, arg);
                return true;
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return false;
            }
        }

        public static bool InvokeVoidAsync(string func, string arg, string arg2, bool showDetails = true)
        {
            Globals.WriteToLog(!showDetails && !Globals.VerboseMode ? $"JS CALL (4): {func}..." : $"JS CALL (4): {func}, {arg}, {arg2}");
            try
            {
                if (ActiveIJsRuntime is null) return false;
                ActiveIJsRuntime.InvokeVoidAsync(func, arg, arg2);
                return true;
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return false;
            }
        }

        public static async Task ReloadPage() => await ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        public static async Task CacheReloadPage() => await ActiveIJsRuntime.InvokeVoidAsync("location.reload(true);");
        #endregion
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
