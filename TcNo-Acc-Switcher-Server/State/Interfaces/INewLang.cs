using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface INewLang
{
    Dictionary<string, string> Strings { get; set; }
    string Current { get; set; }

    /// <summary>
    /// Get a string
    /// </summary>
    string this[string key] { get; }

    /// <summary>
    /// Get a string, and replace variables
    /// </summary>
    /// <param name="key">String to look up</param>
    /// <param name="obj">Object of variables to replace</param>
    string this[string key, object obj] { get; }

    /// <summary>
    /// Tries to load a requested language
    /// </summary>
    /// <param name="lang">Formatted language, example: "en-US"</param>
    bool LoadLang(string lang);

    Dictionary<string, string> GetAvailableLanguagesDict();
    KeyValuePair<string, string> GetCurrentLanguage();
    bool Load(string filename);
}