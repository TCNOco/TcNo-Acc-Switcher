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
    public interface IAppData
    {
        List<SteamUserAcc> SteamUsers { get; set; }
        bool SteamLoadingProfiles { get; set; }
        string WindowTitle { get; set; }
        string CurrentStatus { get; set; }
        string SelectedAccountId { get; }
        Account SelectedAccount { get; set; }
        string SelectedPlatform { get; set; }
        string CurrentSwitcher { get; set; }
        string CurrentSwitcherSafe { get; set; }
        bool FirstMainMenuVisit { get; set; }
        InitializedClasses InitializedClasses { get; set; }
        DiscordRpcClient DiscordClient { get; set; }

        /// <summary>
        /// Contains whether the Client app is running, and the server is it's child.
        /// </summary>
        bool TcNoClientApp { get; set; }

        ObservableCollection<Account> SteamAccounts { get; set; }
        ObservableCollection<Account> BasicAccounts { get; set; }
        bool IsCurrentlyExportingAccounts { get; set; }

        /// <summary>
        /// (Only one time) Checks for update, and submits statistics if enabled.
        /// </summary>
        void FirstLaunchCheck();

        void RefreshDiscordPresenceAsync(bool firstLaunch);
        void RefreshDiscordPresence();
        event Action OnChange;
        void NotifyDataChanged();

        /// <summary>
        /// A general wrapper for InvokeVoidAsync, that returns true if it ran, or false if it doesn't.
        /// </summary>
        Task<bool> InvokeVoidAsync(string func, params object[] args);

        /// <summary>
        /// A general wrapper for InvokeVoid. This will do nothing if ActiveIJsRuntime is null.
        /// </summary>
        Task<TValue> InvokeAsync<TValue>(string func, params object[] args);

        void ReloadPage();
        void CacheReloadPage();
        void ReloadWithToast(string type, string title, string message);
        void NavigateToWithToast(string uri, string type, string title, string message);
        void NavigateTo(string uri, bool forceLoad = false);
        Task NavigateUpOne();
    }

    public class AppData : IAppData
    {
        [Inject] private ILang Lang { get; }
        [Inject] private IAppSettings AppSettings { get; }
        [Inject] private IAppStats AppStats { get; }
        [Inject] private NavigationManager NavManager { get; }
        [Inject] private IJSRuntime JsRuntime { get; }

        public AppData()
        {
            CurrentStatus = Lang["Status_Init"];
        }

        #region First Launch
        // I'm not really sure where to include this for the first page visit, so it is here and called on every page.
        // As far as I understand this works like a browser, and getting it to try and display something may not work the best...
        // So it's called on each page, but only run once.
        private bool FirstLaunch { get; set; } = true;

        #region Accounts
        // These hold accounts for different switchers. Changing settings reloads the Settings, wiping them. This is the second best place to store these.
        public List<SteamUserAcc> SteamUsers { get; set; }
        public bool SteamLoadingProfiles { get; set; }
        #endregion

        /// <summary>
        /// (Only one time) Checks for update, and submits statistics if enabled.
        /// </summary>
        public void FirstLaunchCheck()
        {
            if (!FirstLaunch) return;
            FirstLaunch = false;
            // Check for update in another thread
            // Also submit statistics, if enabled
            new Thread(AppSettings.CheckForUpdate).Start();
            if (AppSettings.StatsEnabled && AppSettings.StatsShare)
                new Thread(AppStats.UploadStats).Start();

            // Discord integration
            RefreshDiscordPresenceAsync(true);

            // Unused. Idk if I will use these, but they are here just in-case.
            //DiscordClient.OnReady += (sender, e) => { Console.WriteLine("Received Ready from user {0}", e.User.Username); };
            //DiscordClient.OnPresenceUpdate += (sender, e) => { Console.WriteLine("Received Update! {0}", e.Presence); };
        }

        public void RefreshDiscordPresenceAsync(bool firstLaunch)
        {
            if (!firstLaunch && DiscordClient.CurrentUser is null) return;
            var dThread = new Thread(RefreshDiscordPresence);
            dThread.Start();
        }

        public void RefreshDiscordPresence()
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
                Logger = new ConsoleLogger { Level = LogLevel.None },
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

        // Window stuff
        private string _windowTitle = "TcNo Account Switcher";

        public string WindowTitle
        {
            get => _windowTitle;
            set
            {
                _windowTitle = value;
                NotifyDataChanged();
                Globals.WriteToLog($"{Environment.NewLine}Window Title changed to: {value}");
            }
        }

        public string CurrentStatus { get; set; }

        public string SelectedAccountId => SelectedAccount.AccountId;
        public Account SelectedAccount { get; set; }
        public string SelectedPlatform { get; set; } = "";

        private string _currentSwitcher = "";
        public string CurrentSwitcher
        {
            get => _currentSwitcher;
            set
            {
                _currentSwitcher = value;
                CurrentSwitcherSafe = Globals.GetCleanFilePath(CurrentSwitcher);
            }
        }

        public string CurrentSwitcherSafe { get; set; } = "";

        public event Action OnChange;
        public void NotifyDataChanged() => OnChange?.Invoke();
        [JsonIgnore] public bool FirstMainMenuVisit { get; set; } = true;
        [JsonIgnore] public InitializedClasses InitializedClasses { get; set; } = new();

        [JsonIgnore] public DiscordRpcClient DiscordClient { get; set; }

        /// <summary>
        /// Contains whether the Client app is running, and the server is it's child.
        /// </summary>
        [JsonIgnore] public bool TcNoClientApp { get; set; }

        #region JS_INTEROP
        /// <summary>
        /// A general wrapper for InvokeVoidAsync, that returns true if it ran, or false if it doesn't.
        /// </summary>
        public async Task<bool> InvokeVoidAsync(string func, params object?[]? args)
        {
            Globals.WriteToLog(!Globals.VerboseMode ? $"JS InvokeVoidAsync: {func}" : $"JS CALL: {func}: {args}");
            try
            {
                if (JsRuntime is null) return false;
                await JsRuntime.InvokeVoidAsync(func, args);
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
        public async Task<TValue> InvokeAsync<TValue>(string func, params object?[]? args)
        {
            Globals.WriteToLog(!Globals.VerboseMode ? $"JS InvokeAsync: {func}" : $"JS CALL: {func}: {args}");
            try
            {
                if (JsRuntime is null) return default;
                return await JsRuntime.InvokeAsync<TValue>(func, args);
            }
            catch (Exception e) when (e is ArgumentNullException or InvalidOperationException or TaskCanceledException
                                          or ArgumentNullException or TaskCanceledException or JSDisconnectedException)
            {
                return default;
            }
        }

        public void ReloadPage() => NavManager.NavigateTo(NavManager.Uri, forceLoad: false);
        public void CacheReloadPage() => NavManager.NavigateTo(NavManager.Uri, forceLoad: true);
        public void ReloadWithToast(string type, string title, string message) =>
            NavManager.NavigateTo($"{NavManager.BaseUri}?toast_type={type}&toast_title={Uri.EscapeDataString(title)}&toast_message={Uri.EscapeDataString(message)}");
        public void NavigateToWithToast(string uri, string type, string title, string message) =>
            NavManager.NavigateTo($"{uri}?toast_type={type}&toast_title={Uri.EscapeDataString(title)}&toast_message={Uri.EscapeDataString(message)}");
        public void NavigateTo(string uri, bool forceLoad = false) => NavManager.NavigateTo(uri, forceLoad);

        public async Task NavigateUpOne()
        {
            var uri = NavManager.Uri;
            if (uri.EndsWith('/')) uri = uri[..^1];
            uri = uri.Replace("http://", "").Replace("https://", "");

            // Navigate up one folder
            if (uri.Contains("/"))
            {
                var split = uri.Split('/');
                var newUri = "http://" + string.Join("/", split.Take(split.Length - 1));
                NavManager.NavigateTo(newUri);
            }
            else
            {
                await InvokeVoidAsync("spinBackButton");
            }
        }
        #endregion


        [JsonIgnore] public ObservableCollection<Account> SteamAccounts { get; set; } = new();
        [JsonIgnore] public ObservableCollection<Account> BasicAccounts { get; set; } = new();

        [JsonIgnore] public bool IsCurrentlyExportingAccounts { get; set; }
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
