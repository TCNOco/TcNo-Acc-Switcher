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

using System.Collections.Generic;

namespace TcNo_Acc_Switcher.State.Interfaces;

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
    /// Tries to load a requested language
    /// </summary>
    /// <param name="lang">Formatted language, example: "en-US"</param>
    bool LoadLang(string lang);

    Dictionary<string, string> GetAvailableLanguagesDict();
    KeyValuePair<string, string> GetCurrentLanguage();
    bool Load(string filename);
}