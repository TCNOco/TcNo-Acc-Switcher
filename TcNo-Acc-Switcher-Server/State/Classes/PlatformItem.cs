using System;
using System.Collections.Generic;
using System.Text.Json.Serialization;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
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
}
