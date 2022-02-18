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
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.General;

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
            set => _instance = value;
        }


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
        public void SetActiveNavMan(NavigationManager nm) => Instance._activeNavMan = nm;

        #region JS_INTEROP
        public static bool InvokeVoidAsync(string func)
        {
            Globals.WriteToLog($"JS CALL (1): {func}");
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func));
        }

        public static bool InvokeVoidAsync(string func, string arg)
        {
            Globals.WriteToLog($"JS CALL (2): {func}, {arg}");
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func, arg));
        }

        public static bool InvokeVoidAsync(string func, object arg)
        {
            Globals.WriteToLog($"JS CALL (3): {func}, {JsonConvert.SerializeObject(arg)}");
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func, arg));
        }

        public static bool InvokeVoidAsync(string func, string arg, string arg2)
        {
            Globals.WriteToLog($"JS CALL (4): {func}, {arg}, {arg2}");
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func, arg, arg2));
        }

        private static bool InvokeVoidAsync(Action func)
        {
            try
            {
                func();
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return false;
            }

            return true;
        }

        public static async Task ReloadPage() => await ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        public static async Task CacheReloadPage() => await ActiveIJsRuntime.InvokeVoidAsync("location.reload(true);");
        #endregion
    }
}
