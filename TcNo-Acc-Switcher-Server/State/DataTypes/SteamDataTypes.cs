using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Server.State.DataTypes
{
    /// <summary>
    /// Simple class for storing info related to Steam account, for switching and displaying.
    /// </summary>
    public class SteamUser
    {
        [JsonIgnore] public string SteamId { get; set; }
        [JsonProperty("AccountName", Order = 0)] public string AccName { get; set; }
        [JsonProperty("PersonaName", Order = 1)] public string Name { get; set; }
        [JsonProperty("RememberPassword", Order = 2)] public string RememberPass = "1"; // Should always be 1
        [JsonProperty("mostrecent", Order = 3)] public string MostRec = "0";
        [JsonProperty("Timestamp", Order = 4)] public string LastLogin { get; set; }
        [JsonProperty("WantsOfflineMode", Order = 5)] public string OfflineMode = "0";
        [JsonIgnore] public string ImageDownloadUrl { get; set; }
        [JsonIgnore] public string ImgUrl { get; set; }
        [JsonIgnore] public bool BanInfoLoaded { get; set; }
        [JsonIgnore] public bool Vac { get; set; }
        // Either Limited or Community banned status (when using Steam Web API)
        [JsonIgnore] public bool Limited { get; set; }
    }

    /// <summary>
    /// Class for storing SteamID, VAC status and Limited status.
    /// </summary>
    public class VacStatus
    {
        [JsonProperty(Order = 0)] public string SteamId { get; set; }
        [JsonProperty(Order = 1)] public bool Vac { get; set; }
        [JsonProperty(Order = 2)] public bool Ltd { get; set; }
    }
}
