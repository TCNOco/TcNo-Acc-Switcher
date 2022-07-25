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
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Classes.Templated;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class TemplatedPlatformState : ITemplatedPlatformState
{
    private readonly IAppState _appState;
    private readonly IGameStats _gameStats;
    private readonly IModals _modals;
    private readonly ISharedFunctions _sharedFunctions;
    private readonly IStatistics _statistics;
    private readonly IToasts _toasts;
    private readonly IWindowSettings _windowSettings;

    public List<string> AvailablePlatforms { get; set; }
    public TemplatedPlatformContextMenu ContextMenu { get; set; }

    public Platform CurrentPlatform { get; set; }
    public List<Platform> Platforms { get; set; }

    private bool _isInit;

    // Replace the other PlatformItem.cs here
    // Just have this load all the names and identifiers as well.

    private readonly string _platformJsonPath = Path.Join(Globals.AppDataFolder, "Platforms.json");
    public TemplatedPlatformState(IAppState appState, IGameStats gameStats, IModals modals,
        ISharedFunctions sharedFunctions, IStatistics statistics,
        IToasts toasts, IWindowSettings windowSettings)
    {
        _appState = appState;
        _gameStats = gameStats;
        _modals = modals;
        _sharedFunctions = sharedFunctions;
        _statistics = statistics;
        _toasts = toasts;
        _windowSettings = windowSettings;
    }

    public void LoadTemplatedPlatformState(ITemplatedPlatformFuncs templatedPlatformFuncs)
    {
        if (_isInit) return;
        _isInit = true;

        if (!File.Exists(_platformJsonPath))
        {
            _toasts.ShowToastLang(ToastType.Error, "Toast_FailedPlatformsLoad");
            Globals.WriteToLog("Failed to locate Platforms.json! This will cause a lot of features to break.");
            return;
        }

        Platforms = JsonConvert.DeserializeObject<List<Platform>>(File.ReadAllText(_platformJsonPath));
        if (Platforms is null) return;
        AvailablePlatforms = Platforms.Select(x => x.Name).ToList();

        // Import list to master platform list -- For displaying on the main menu.
        foreach (var plat in Platforms)
        {
            // Add to master platform list
            if (_windowSettings.Platforms.Count(y => y.Name == plat.Name) == 0)
                _windowSettings.Platforms.Add(new PlatformItem(plat.Name, plat.Identifiers, plat.ExeName, false));
            else
            {
                // Make sure that everything is set up properly.
                var platform = _windowSettings.Platforms.First(y => y.Name == plat.Name);
                platform.SetFromPlatformItem(new PlatformItem(plat.Name, plat.Identifiers, plat.ExeName, false));
            }
        }

        Task.Run(() =>
        {
            ContextMenu = new TemplatedPlatformContextMenu(_appState, _gameStats, _modals, templatedPlatformFuncs, this, _toasts);
        });
    }

    public async Task SetCurrentPlatform(ITemplatedPlatformSettings templatedPlatformSettings, string platformName)
    {
        CurrentPlatform = Platforms.First(x => x.Name == platformName || x.Identifiers.Contains(platformName));
        CurrentPlatform.InitAfterDeserialization();
        //_templatedPlatformSettings = new TemplatedPlatformSettings(); // Load saved settings
        templatedPlatformSettings.LoadTemplatedPlatformSettings();

        _appState.Switcher.CurrentSwitcher = CurrentPlatform.Name;
        _appState.Switcher.TemplatedAccounts.Clear();
        await LoadAccounts();
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
        _statistics.SetAccountCount(CurrentPlatform.SafeName, accList.Count);

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
            var acc = _appState.Switcher.TemplatedAccounts.FirstOrDefault(x => x.AccountId == key);
            if (acc is null) return;
            acc.Note = val;
        }

        _modals.IsShown = false;
    }


    /// <summary>
    /// Iterate through account list and insert into platforms account screen
    /// </summary>
    /// <param name="accList">Account list</param>
    private async Task InsertAccounts(List<string> accList)
    {
        LoadAccountIds();

        _appState.Switcher.TemplatedAccounts.Clear();

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
            account.UserStats = _gameStats.GetUserStatsAllGamesMarkup(str);

            _appState.Switcher.TemplatedAccounts.Add(account);
        }
        await _sharedFunctions.FinaliseAccountList(); // Init context menu & Sorting
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
    public Dictionary<string, string> AccountIds { get; set; }
    public void LoadAccountIds() => AccountIds = Globals.ReadDict(CurrentPlatform.IdsJsonPath);
    public void SaveAccountIds() =>
        File.WriteAllText(CurrentPlatform.IdsJsonPath, JsonConvert.SerializeObject(AccountIds));
    public string GetNameFromId(string accId) => AccountIds.ContainsKey(accId) ? AccountIds[accId] : accId;
    #endregion
}