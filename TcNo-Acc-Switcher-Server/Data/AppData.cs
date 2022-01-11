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

        public List<string> OldPlatformList = new() {"Steam", "Origin", "Ubisoft", "BattleNet", "Riot", "Discord"};
        private List<string> _platformList;
        public List<string> PlatformList
        {
            get
            {
                _instance._platformList = new List<string> { "Steam", "Origin", "Ubisoft", "BattleNet", "Riot", "Discord" };

                // Add enabled basic platforms:
                _instance._platformList =
                    _instance._platformList.Union(AppSettings.Instance.EnabledBasicPlatforms).ToList();
                return _instance._platformList;
            }
            set => _instance._platformList = value;
        }
        #endregion

        public event Action OnChange;

        private void NotifyDataChanged() => OnChange?.Invoke();

        private IJSRuntime _activeIJsRuntime;
        [JsonIgnore] public static IJSRuntime ActiveIJsRuntime { get => _instance._activeIJsRuntime; set => _instance._activeIJsRuntime = value; }
        public void SetActiveIJsRuntime(IJSRuntime jsr) => _instance._activeIJsRuntime = jsr;

        private NavigationManager _activeNavMan;
        [JsonIgnore] public static NavigationManager ActiveNavMan { get => _instance._activeNavMan; set => _instance._activeNavMan = value; }
        public void SetActiveNavMan(NavigationManager nm) => _instance._activeNavMan = nm;

        #region JS_INTEROP
        public static bool InvokeVoidAsync(string func)
        {
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func));
        }

        public static bool InvokeVoidAsync(string func, string arg)
        {
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func, arg));
        }

        public static bool InvokeVoidAsync(string func, object arg)
        {
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func, arg));
        }

        public static bool InvokeVoidAsync(string func, string arg, string arg2)
        {
            return ActiveIJsRuntime is not null && InvokeVoidAsync(async () => await ActiveIJsRuntime.InvokeVoidAsync(func, arg, arg2));
        }

        private static bool InvokeVoidAsync(Action func)
        {
            try
            {
                func();
            }
            catch (ArgumentNullException)
            {
                return false;
            }
            catch (InvalidOperationException)
            {
                return false;
            }
            catch (TaskCanceledException)
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
