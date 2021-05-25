// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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

using Newtonsoft.Json;


namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class Index
    {
        /// <summary>
        /// Simple class for storing info related to Steam account, for switching and displaying.
        /// </summary>
        public class Steamuser
        {
            [JsonIgnore] public string SteamId { get; set; }
            [JsonProperty("AccountName", Order = 0)] public string AccName { get; set; }
            [JsonProperty("PersonaName", Order = 1)] public string Name { get; set; }
            [JsonProperty("RememberPassword", Order = 2)] public string RememberPass = "1"; // Should always be 1
            [JsonProperty("mostrecent", Order = 3)] public string MostRec = "0";
            [JsonProperty("Timestamp", Order = 4)] public string LastLogin { get; set; }
            [JsonProperty("WantsOfflineMode", Order = 5)] public string OfflineMode = "0";
            [JsonIgnore] public string ImgUrl { get; set; }
        }
    }
}
