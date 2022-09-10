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
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.State.Classes.Templated;

/// <summary>
/// Loads and holds info for one platform from Platforms.json
/// </summary>
// Root myDeserializedClass = JsonConvert.DeserializeObject<Root>(myJsonResponse);
public class Platform
{
    [JsonProperty(Required = Required.Always)]
    public string Name { get; set; } = "";
    [JsonProperty(Required = Required.Always)]
    public List<string> Identifiers { get; set; } = new();

    [JsonProperty(Required = Required.Always)]
    public string ExeLocationDefault { get; set; } = "";
    [JsonProperty(Required = Required.Always)]
    public Dictionary<string, string> LoginFiles { get; set; } = new();
    public List<string> ExesToEnd { get; set; } = new();
    public List<string> PathListToClear { get; set; } = new();
    public string UniqueIdFile { get; set; } = "";
    public string UniqueIdFolder { get; set; } = "";
    public string UniqueIdRegex { get; set; } = "";
    public string UniqueIdMethod { get; set; } = "";
    public Extras Extras { get; set; } = new();
    public string ExeExtraArgs { get; set; } = "";
    public bool ExitBeforeInteract { get; set; }
    public bool RegDeleteOnClear { get; set; }
    public bool ClearLoginCache { get; set; } = true;


    // Generated from data
    public string PrimaryId;
    [JsonIgnore] public bool HasExtras { get; set; }
    [JsonIgnore] public string ExeName { get; set; }
    [JsonIgnore] public bool HasRegistryFiles { get; set; }

    // Data used elsewhere
    [JsonIgnore] public string SafeName { get; set; }
    [JsonIgnore] public string SettingsFile { get; set; }
    [JsonIgnore] public string PlatformLoginCache { get; set; }
    [JsonIgnore] public string ShortcutFolder { get; set; }
    [JsonIgnore] public string ShortcutImageFolder { get; set; }
    [JsonIgnore] public string ShortcutImagePath { get; set; }
    [JsonIgnore] public string UniqueFilePath { get; set; }
    [JsonIgnore] public string IdsJsonPath { get; set; }
    [JsonIgnore] public bool IsInit { get; set; }
    [JsonIgnore] public MarkupString UserModalExtraButtons { get; set; }
    [JsonIgnore] public string UserModalCopyText { get; set; }

    public void InitAfterDeserialization()
    {
        // Generate some vars
        PrimaryId = Identifiers.First();
        SafeName = Globals.GetCleanFilePath(Name);
        ExeName = Path.GetFileName(ExeLocationDefault);
        SettingsFile = Path.Join("Settings\\", SafeName + ".json");
        PlatformLoginCache = $"LoginCache\\{SafeName}\\";
        IdsJsonPath = Path.Join(PlatformLoginCache, "ids.json");
        ShortcutFolder = $"LoginCache\\{SafeName}\\Shortcuts\\";
        ShortcutImageFolder = $"img\\shortcuts\\{SafeName}\\";
        ShortcutImagePath = Path.Join(Globals.UserDataFolder, "wwwroot\\", ShortcutImageFolder);
        UniqueFilePath = ExpandEnvironmentVariables(UniqueIdFile);

        if (Extras is not null && Extras.UsernameModalExtraButtons != "")
            UserModalExtraButtons = new MarkupString(Globals.ReadAllText(Path.Join(Globals.AppDataFolder, Extras.UsernameModalExtraButtons)));
        if (UserModalCopyText != "")
            UserModalCopyText = Globals.ReadAllText(Path.Join(Globals.AppDataFolder, UserModalCopyText));

        // Expand paths and variables
        ExeLocationDefault = ExpandEnvironmentVariables(ExeLocationDefault);
        var loginFilesKeys = new List<string>(LoginFiles.Keys);
        foreach (var f in loginFilesKeys)
        {
            LoginFiles[f] = ExpandEnvironmentVariables(LoginFiles[f]);
            if (f.Contains("REG:")) HasRegistryFiles = true;
        }
        if (PathListToClear is not null && PathListToClear.Contains("SAME_AS_LOGIN_FILES"))
            PathListToClear = LoginFiles.Keys.ToList();

        // Check all variables in Extras:
        if (Extras is not null && Extras != new Extras()) HasExtras = true;
        else return;

        // Then the saved settings need to be loaded. This is run just after.

        IsInit = true;
    }

    public string GetShortcutIgnoredPath(string shortcut) =>
        Path.Join(ShortcutFolder, shortcut.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));
    public string AccountLoginCachePath(string acc) =>
        Path.Join(PlatformLoginCache, $"{acc}\\");
    public string GetShortcutImagePath(string gameShortcutName) =>
        Path.Join(ShortcutImageFolder, Globals.RemoveShortcutExt(gameShortcutName) + ".png");
    public Dictionary<string, string> ReadRegJson(string acc) =>
        Globals.ReadDict(Path.Join(AccountLoginCachePath(acc), "reg.json"), true);
    public void SaveRegJson(Dictionary<string, string> regJson, string acc)
    {
        if (regJson.Count == 0) return;
        var path = Path.Join(AccountLoginCachePath(acc), "reg.json");
        var outText = JsonConvert.SerializeObject(regJson);
        if (outText.Length < 4 && File.Exists(path))
            Globals.DeleteFile(path);
        else
            File.WriteAllText(path, outText);
    }

    public string GetUserModalHintText(ILang lang)
    {
        try
        {
            return lang[Extras.UsernameModalHintText];
        }
        catch (Exception)
        {
            return Extras.UsernameModalHintText;
        }
    }

    /// <summary>
    /// Expands custom environment variables.
    /// </summary>
    /// <param name="path"></param>
    /// <returns></returns>
    public string ExpandEnvironmentVariables(string path)
    {
        path = Globals.ExpandEnvironmentVariables(path);
        path = path.Replace("%Platform_Folder%", ExeLocationDefault ?? "");
        return path;
    }
}

public class Extras
{
    public List<string> CachePaths { get; set; }
    public Dictionary<string, string> BackupFolders { get; set; }
    public string UsernameModalExtraButtons { get; set; } = "";
    public string UsernameModalCopyText { get; set; } = "";
    public string UsernameModalHintText { get; set; } = "";
    public bool ShortcutIncludeMainExe { get; set; } = true;
    public bool SearchStartMenuForIcon { get; set; }
    public List<string> ShortcutFolders { get; set; } = new();
    public List<string> ShortcutIgnore { get; set; } = new();
    public string ProfilePicFromFile { get; set; } = "";
    public string ProfilePicRegex { get; set; } = "";
    public string ProfilePicPath { get; set; } = "";
    public Dictionary<string, string> BackupPaths { get; set; } = new();
    public List<string> BackupFileTypesIgnore { get; set; } = new();
    public List<string> BackupFileTypesInclude { get; set; } = new();
    public string ClosingMethod { get; set; } = "Combined";
    public string StartingMethod { get; set; } = "Default";
    public string RegDeleteOnClear { get; set; }
}