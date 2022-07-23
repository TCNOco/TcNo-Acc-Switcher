using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using HtmlAgilityPack;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes.GameStats;

/// <summary>
/// A single game's statistics, for use with others on a platform.
/// </summary>
public class GameStatSaved
{
    [Inject] private Lang Lang { get; set; }
    [Inject] private GameStatsRoot GameStatsRoot { get; set; }
    [Inject] private IAppState AppState { get; set; }
    [Inject] private Toasts Toasts { get; set; }
        
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
        UniqueId = GameStatsRoot.StatsDefinitions[game].UniqueId;
        Indicator = GameStatsRoot.StatsDefinitions[game].Indicator;
        Url = GameStatsRoot.StatsDefinitions[game].Url;
        Cookies = GameStatsRoot.StatsDefinitions[game].RequestCookies;

        RequiredVars = GameStatsRoot.StatsDefinitions[game].Vars;
        //foreach (var (k, v) in jGame["Vars"]?.ToObject<Dictionary<string, string>>()!)
        //{
        //    Vars.Add(k, v);
        //}

        ToCollect = GameStatsRoot.StatsDefinitions[game].Collect;

        if (ToCollect != null)
            foreach (var collectInstruction in ToCollect.Where(collectInstruction => collectInstruction.Key == "%PROFILEIMAGE%"))
                collectInstruction.Value.ToggleText = Lang["ProfileImage_ToggleText"];

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
    public async Task<bool> SetAccount(Dictionary<string, string> vars, List<string> hiddenMetrics)
    {
        if (CachedStats.ContainsKey(AppState.Switcher.SelectedAccountId))
        {
            CachedStats[AppState.Switcher.SelectedAccountId].Vars = vars;
            CachedStats[AppState.Switcher.SelectedAccountId].HiddenMetrics = hiddenMetrics;
        }
        else
            CachedStats[AppState.Switcher.SelectedAccountId] = new UserGameStat() { Vars = vars, HiddenMetrics = hiddenMetrics };

        if (CachedStats[AppState.Switcher.SelectedAccountId].Collected.Count == 0 || DateTime.Now.Subtract(CachedStats[AppState.Switcher.SelectedAccountId].LastUpdated).Days >= 1)
        {
            Toasts.ShowToastLang(ToastType.Info, "Toast_LoadingStats");
            _lastLoadingNotification = DateTime.Now;
            return await LoadStatsFromWeb(AppState.Switcher.SelectedAccountId, AppState.Switcher.CurrentSwitcher);
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
            Toasts.ShowToastLang(ToastType.Info, "Toast_LoadingStats");
            _lastLoadingNotification = DateTime.Now;
        }

        // Read doc from web
        HtmlDocument doc = new();
        if (!Globals.GetWebHtmlDocument(ref doc, UrlSubbed(userStat), out var responseText, Cookies))
        {
            Toasts.ShowToastLang(ToastType.Error, new LangSub("Toast_GameStatsLoadFail", new { Game }));
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
            Toasts.ShowToastLang(ToastType.Error, new LangSub("Toast_GameStatsEmpty", new { AccoundId = Globals.GetCleanFilePath(accountId), Game = Globals.GetCleanFilePath(Game) }));
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