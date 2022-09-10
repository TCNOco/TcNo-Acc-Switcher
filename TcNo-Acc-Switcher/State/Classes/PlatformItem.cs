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
using System.Text.Json.Serialization;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher.State.Classes;

public class PlatformItem : IComparable
{
    public string Name = "";
    public bool Enabled;
    public int DisplayIndex = 99;
    [JsonIgnore] public string SafeName = "";
    [JsonIgnore] public string NameNoSpace = "";
    [JsonIgnore] public string Identifier = "";
    [JsonIgnore] public string ExeName = "";
    [JsonIgnore] public List<string> PossibleIdentifiers = new(); // Other identifiers that can refer to this platform. (b, bnet, battlenet, etc)
    public int CompareTo(object o)
    {
        var a = this;
        var b = (PlatformItem)o;
        return string.CompareOrdinal(a.Name, b.Name);
    }

    // Needed for JSON serialization/deserialization
    public PlatformItem() { }

    // Used for first init. The rest of the info is added by BasicPlatforms.
    public PlatformItem(string name, bool enabled)
    {
        Name = name;
        SafeName = Globals.GetCleanFilePath(name);
        NameNoSpace = SafeName.Replace(" ", "");
        Enabled = enabled;
    }
    public PlatformItem(string name, List<string> identifiers, string exeName, bool enabled)
    {
        Name = name;
        SafeName = Globals.GetCleanFilePath(name);
        NameNoSpace = SafeName.Replace(" ", "");
        Enabled = enabled;
        Identifier = identifiers[0];
        PossibleIdentifiers = identifiers;
        ExeName = exeName;
    }

    /// <summary>
    /// Set from a new PlatformItem - Not including Enabled.
    /// </summary>
    public void SetFromPlatformItem(PlatformItem inItem)
    {
        Name = inItem.Name;
        SafeName = Globals.GetCleanFilePath(Name);
        NameNoSpace = SafeName.Replace(" ", "");
        Identifier = inItem.Identifier;
        ExeName = inItem.ExeName;
    }

    public void SetEnabled(bool enabled)
    {
        Enabled = enabled;
        // Sort after changing these.
    }
}