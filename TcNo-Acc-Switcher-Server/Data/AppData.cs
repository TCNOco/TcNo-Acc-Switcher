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
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;

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

        }
        #endregion

        #region Accounts
        // These hold accounts for different switchers. Changing settings reloads the Settings, wiping them. This is the second best place to store these.

        private JToken _bNetAccountsList;
        public static JToken BNetAccountsList { get => Instance._bNetAccountsList; set => Instance._bNetAccountsList = value; }

        private List<Pages.Steam.Index.Steamuser> _steamUsers;
        public static List<Pages.Steam.Index.Steamuser> SteamUsers { get => Instance._steamUsers; set => Instance._steamUsers = value; }

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

        public List<string> OldPlatformList = new() { "Steam", "BattleNet" };
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

        public List<string> SortedPlatformList()
        {
            return GenericFunctions.OrderAccounts(Instance.PlatformList, "Settings\\platformOrder.json");
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

        public static bool InvokeVoidAsync(string func, string arg)
        {
            Globals.WriteToLog($"JS CALL (2): {func}, {arg}");
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

        public static bool InvokeVoidAsync(string func, object arg)
        {
            Globals.WriteToLog($"JS CALL (3): {func}, {JsonConvert.SerializeObject(arg)}");
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

        public static bool InvokeVoidAsync(string func, string arg, string arg2)
        {
            Globals.WriteToLog($"JS CALL (4): {func}, {arg}, {arg2}");
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
            BattleNet = false;
        }
        public bool Basic { get; set; }
        public bool Steam { get; set; }
        public bool BattleNet { get; set; }
    }
}
