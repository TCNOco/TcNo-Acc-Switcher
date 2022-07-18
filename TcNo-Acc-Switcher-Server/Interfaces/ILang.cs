using System.Collections.Generic;

namespace TcNo_Acc_Switcher_Server.Interfaces;

public interface ILang
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
    /// Loads the programs default language: English.
    /// </summary>
    void LoadDefault();

    /// <summary>
    /// Loads the system's language, or the user's saved language
    /// </summary>
    void LoadLocalized();

    /// <summary>
    /// Tries to load a requested language
    /// </summary>
    /// <param name="lang">Formatted language, example: "en-US"</param>
    bool LoadLang(string lang);

    /// <summary>
    /// Get list of files in Resources folder
    /// </summary>
    List<string> GetAvailableLanguages();

    Dictionary<string, string> GetAvailableLanguagesDict();
    KeyValuePair<string, string> GetCurrentLanguage();
    bool Load(string filename, bool save = false);
}