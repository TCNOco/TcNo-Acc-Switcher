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
using HtmlAgilityPack;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.Pages.General;
using TcNo_Acc_Switcher.State.DataTypes;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.State.Classes.GameStats;

/// <summary>
/// A single game's statistics, for use with others on a platform.
/// </summary>
public class GameStatSaved
{
    private readonly IAppState _appState;
    private readonly IGameStatsRoot _gameStatsRoot;
    private readonly ILang _lang;
    private readonly IToasts _toasts;

    public GameStatSaved(ILang lang, IGameStatsRoot gameStatsRoot, IAppState appState, IToasts toasts)
    {
        _lang = lang;
        _gameStatsRoot = gameStatsRoot;
        _appState = appState;
        _toasts = toasts;
    }

    public SortedDictionary<string, UserGameStat> Stats { get; set; }


    #region Properties

    private DateTime _lastLoadingNotification = DateTime.MinValue;

    public bool IsInit;
    public string Game { get; set; }
    public string UniqueId { get; set; } = "";
    public string Indicator { get; set; } = "";
    public string Url { get; set; } = "";
    public string Cookies { get; set; } = "";

    /// <summary>
    ///Dictionary of Variable:Values for each account, for use in statistics web url. Such as Username="Kevin".
    /// </summary>
    public Dictionary<string, string> RequiredVars = new();
    /// <summary>
    /// Dictionary of StatName:CollectInstructions for collecting stats from web and info from user
    /// </summary>
    public Dictionary<string, CollectInstruction> ToCollect = new();


    private readonly string _cacheFileFolder = Path.Join(Globals.UserDataFolder, "StatsCache");
    private string CacheFilePath => Path.Join(_cacheFileFolder, Globals.GetCleanFilePath($"{Game}.json"));
    /// <summary>
    /// Dictionary of AccountId:UserGameStats
    /// </summary>
    public SortedDictionary<string, UserGameStat> CachedStats = new();
    #endregion


    public async Task SetGameStat(string game)
    {
        Game = game;
        UniqueId = _gameStatsRoot.StatsDefinitions[game].UniqueId;
        Indicator = _gameStatsRoot.StatsDefinitions[game].Indicator;
        Url = _gameStatsRoot.StatsDefinitions[game].Url;
        Cookies = _gameStatsRoot.StatsDefinitions[game].RequestCookies;

        RequiredVars = _gameStatsRoot.StatsDefinitions[game].Vars;
        //foreach (var (k, v) in jGame["Vars"]?.ToObject<Dictionary<string, string>>()!)
        //{
        //    Vars.Add(k, v);
        //}

        ToCollect = _gameStatsRoot.StatsDefinitions[game].Collect;

        if (ToCollect != null)
            foreach (var collectInstruction in ToCollect.Where(collectInstruction => collectInstruction.Key == "%PROFILEIMAGE%"))
                collectInstruction.Value.ToggleText = _lang["ProfileImage_ToggleText"];

        //foreach (var (k, v) in jGame["Collect"]?.ToObject<Dictionary<string, CollectInstruction>>()!)
        //{
        //    Collect.Add(k, v);
        //}

        await LoadStats();

        IsInit = true;
    }

    /// <summary>
    /// Set up new accounts. Set game name if you want all accounts to save after setting values (Recommended).
    /// </summary>
    public async Task<bool> SetAccount(Dictionary<string, string> vars, List<string> metrics)
    {
        if (CachedStats.ContainsKey(_appState.Switcher.SelectedAccountId))
        {
            CachedStats[_appState.Switcher.SelectedAccountId].Vars = vars;
            CachedStats[_appState.Switcher.SelectedAccountId].Metrics = metrics;
        }
        else
            CachedStats[_appState.Switcher.SelectedAccountId] = new UserGameStat() { Vars = vars, Metrics = metrics };

        if (CachedStats[_appState.Switcher.SelectedAccountId].Collected.Count == 0 || DateTime.Now.Subtract(CachedStats[_appState.Switcher.SelectedAccountId].LastUpdated).Days >= 1)
        {
            await _toasts.ShowToastLangAsync(ToastType.Info, "Toast_LoadingStats");
            _lastLoadingNotification = DateTime.Now;
            return await LoadStatsFromWeb(_appState.Switcher.SelectedAccountId, _appState.Switcher.CurrentSwitcher);
        }
        else
            SaveStats();

        return true;
    }

    /// <summary>
    /// Return URL with saved vars subbed in
    /// </summary>
    private string UrlSubbed(UserGameStat userStat)
    {
        var completeUrl = Url;
        foreach (var (key, value) in userStat.Vars)
        {
            completeUrl = completeUrl.Replace($"{{{key}}}", value);
        }
        return completeUrl;
    }

    /// <summary>
    /// Collects user statistics for account
    /// </summary>
    /// <returns>Successful or not</returns>
    public async Task<bool> LoadStatsFromWeb(string accountId, string platform = "")
    {
        // Check if init, and list of accounts contains Id
        if (!IsInit || !CachedStats.ContainsKey(accountId)) return false;
        var userStat = CachedStats[accountId];

        // Check if account has required variables set.
        if (userStat.Vars.Count == 0) return false;

        // Notify user than an account is being loaded - >5 seconds apart.
        if (DateTime.Now.Subtract(_lastLoadingNotification).Seconds >= 5)
        {
            await _toasts.ShowToastLangAsync(ToastType.Info, "Toast_LoadingStats");
            _lastLoadingNotification = DateTime.Now;
        }

        // Read doc from web
        HtmlDocument doc = new();
        if (!Globals.GetWebHtmlDocument(ref doc, UrlSubbed(userStat), out var responseText, Cookies))
        {
            await _toasts.ShowToastLangAsync(ToastType.Error, new LangSub("Toast_GameStatsLoadFail", new { Game }));
            return false;
        }

        // Collect each requested element
        foreach (var (itemName, ci) in ToCollect)
        {
            var text = "";
            HtmlNode xPathResponse;
            try
            {
                xPathResponse = doc.DocumentNode.SelectSingleNode(ci.XPath);
            }
            catch (System.Xml.XPath.XPathException e)
            {
                Globals.WriteToLog($"Failed to collect '{itemName}' statistic for '{Game}' ({ci.XPath})", e);
                continue;
            }

            if (xPathResponse is null) continue;

            text = ci.Select switch
            {
                "innerText" => xPathResponse.InnerText,
                "innerHtml" => xPathResponse.InnerHtml,
                "attribute" => xPathResponse.Attributes[ci.SelectAttribute]?.Value ?? "",
                _ => text
            };

            // Don't display if equal to default
            if (text == ci.NoDisplayIf) continue;

            // Handle special items
            if (itemName == "%PROFILEIMAGE%")
            {
                if (platform != "")
                {
                    Globals.DownloadProfileImage(platform, accountId, text, Globals.WwwRoot);
                }
                continue;
            }

            // And special Special Types
            if (ci.SpecialType == "ImageDownload")
            {
                var fileName = $"{Indicator}-{itemName}-{accountId}.jpg";
                var imgPath = Path.Join(Globals.WwwRoot, "img\\statsCache\\" + fileName);
                if (!await Globals.DownloadFileAsync(text, imgPath))
                {
                    continue;
                }
                text = "img/statsCache/" + fileName;
            }

            // Else, format as requested
            text = ci.DisplayAs.Replace("%x%", text);
            userStat.Collected[itemName] = text;
        }

        userStat.LastUpdated = DateTime.Now;

        if (!userStat.Collected.Any())
        {
            Directory.CreateDirectory(Path.Join(Globals.UserDataFolder, "temp"));
            await File.WriteAllTextAsync(Path.Join(Globals.UserDataFolder, "temp", $"download-{Globals.GetCleanFilePath(accountId)}-{Globals.GetCleanFilePath(Game)}.html"), responseText);
            await _toasts.ShowToastLangAsync(ToastType.Error, new LangSub("Toast_GameStatsEmpty", new { AccoundId = Globals.GetCleanFilePath(accountId), Game = Globals.GetCleanFilePath(Game) }));
            if (CachedStats.ContainsKey(accountId))
                CachedStats.Remove(accountId);
            return false;
        }


        CachedStats[accountId] = userStat;
        SaveStats();
        return true;
    }

    /// <summary>
    /// Save all game stats for all accounts on current platform
    /// </summary>
    public void SaveStats()
    {
        _ = Directory.CreateDirectory(_cacheFileFolder);
        File.WriteAllText(CacheFilePath, JsonConvert.SerializeObject(CachedStats));
    }

    /// <summary>
    /// Load all game stats for all accounts on current platform
    /// </summary>
    public async Task LoadStats()
    {
        _ = Directory.CreateDirectory(_cacheFileFolder);

        if (!File.Exists(CacheFilePath)) return;
        var jCachedStats = GeneralFuncs.LoadSettings(CacheFilePath);
        CachedStats = jCachedStats.ToObject<SortedDictionary<string, UserGameStat>>() ?? new SortedDictionary<string, UserGameStat>();

        // Check for outdated items, and refresh them
        foreach (var id in CachedStats.Keys.Where(id => DateTime.Now.Subtract(CachedStats[id].LastUpdated).Days >= 1))
        {
            await LoadStatsFromWeb(id);
        }
    }
}