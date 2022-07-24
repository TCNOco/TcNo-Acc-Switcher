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
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text.Json;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.Interfaces;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace TcNo_Acc_Switcher_Server.State;

/// <summary>
/// NOTE: This "Lang" is temporary, and will replace the old Lang later.
/// The old method does not use Singleton instances properly -- Or even at all!
/// This should be much better when it's set up and replaced everything.
/// For now, having 2 loaded in memory should not be an issue.
/// </summary>
public class Lang : ILang
{
    private readonly IWindowSettings _windowSettings;

    /// <summary>
    /// Loads the default en-US language on creation.
    /// Any strings missing from other languages will use the en-US fallback.
    /// </summary>
    public Lang(IWindowSettings windowSettings)
    {
        _windowSettings = windowSettings;

        Load("en-US");

        // If language is not set, try and load from the user's language.
        // Or load the user's language
        Load(_windowSettings.Language == "" ? CultureInfo.CurrentCulture.Name : _windowSettings.Language);
    }

    public Dictionary<string, string> Strings { get; set; } = new();
    public string Current { get; set; } = "";

    /// <summary>
    /// Get a string
    /// </summary>
    public string this[string key] => Strings.ContainsKey(key) ? Strings[key] : key;

    /// <summary>
    /// Get a string, and replace variables
    /// </summary>
    /// <param name="key">String to look up</param>
    /// <param name="obj">Object of variables to replace</param>
    public string this[string key, object obj]
    {
        get
        {
            try
            {
                if (!Strings.ContainsKey(key)) return key;
                var s = Strings[key];
                if (obj is JsonElement)
                    foreach (var (k, v) in JObject.FromObject(JsonConvert.DeserializeObject(obj.ToString()!)!))
                    {
                        if (v != null) s = s.Replace($"{{{k}}}", v.Value<string>());
                    }
                else
                    foreach (var pi in obj.GetType().GetProperties())
                    {
                        dynamic val = pi.GetValue(obj, null);
                        if (val is int) val = val.ToString();
                        s = s.Replace($"{{{pi.Name}}}", (string)val);
                    }
                return s;
            }
            catch (NullReferenceException e)
            {
                Globals.WriteToLog(e);
                return "[Failed to get text: missing parameter] " + key;
            }
        }
    }

    #region FILE_HANDLING
    /// <summary>
    /// Tries to load a requested language
    /// </summary>
    /// <param name="lang">Formatted language, example: "en-US"</param>
    public bool LoadLang(string lang)
    {
        Load("en-US");
        return Load(lang);
    }

    /// <summary>
    /// Get list of files in Resources folder
    /// </summary>
    public readonly List<string> AvailableLanguages = Directory.GetFiles(Path.Join(Globals.AppDataFolder, "Resources")).Select(f => Path.GetFileName(f).Split(".yml")[0]).ToList();
    public Dictionary<string, string> GetAvailableLanguagesDict()
    {
        var dict = AvailableLanguages.ToDictionary(l => new CultureInfo(l).DisplayName);
        dict.Remove("English (Portugal)");
        dict["English (Pirate)"] = "en-PT";
        return dict;
    }

    public KeyValuePair<string, string> GetCurrentLanguage()
    {
        if (Current == "en-PT") return new KeyValuePair<string, string>("English (Pirate)", Current);
        return new KeyValuePair<string, string>(new CultureInfo(Current).DisplayName, Current);
    }

    public bool Load(string filename)
    {
        try
        {
            var path = Path.Join(Globals.AppDataFolder, "Resources", filename + ".yml");

            // If the path doesn't exist: Find the closest available language.
            if (!File.Exists(path))
            {
                // Get closest available language
                var langGroup = filename.Split('-')[0];
                var foundClose = false;
                foreach (var l in AvailableLanguages.Where(l => l.StartsWith(langGroup)))
                {
                    // A close language was found, and was set.
                    path = Path.Join(Globals.AppDataFolder, "Resources", l + ".yml");
                    foundClose = true;
                    Current = l;
                    _windowSettings.Language = filename;
                    _windowSettings.Save();
                    break;
                }

                // If could not find a language close to requested.
                // The current language will remain English
                if (!foundClose)
                    return false;
            }

            // Load from en-EN file into Strings
            var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
            var text = Globals.ReadAllText(path);
            var dict = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(text)));
            Debug.Assert(dict != null, nameof(dict) + " != null"); // These files have to exist, or the program will break in many ways
            foreach (var (k, v) in dict)
            {
                Strings[k] = v;
            }

            Current = filename;

            // Save the language if it has changed.
            if (_windowSettings.Language == filename) return true;
            _windowSettings.Language = filename;
            _windowSettings.Save();
            return true;
        }
        catch (Exception e)
        {
            Globals.WriteToLog("Can not load language information! See log for more info!", e);
            return false;
        }
    }
    #endregion
}
public class LangSub
{
    public string LangKey { get; set; }
    public object Variable { get; set; }

    public LangSub(string key, object var)
    {
        LangKey = key;
        Variable = var;
    }
}