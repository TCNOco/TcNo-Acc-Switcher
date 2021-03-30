using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class Index : ComponentBase
    {
        [Inject]
        protected IJSRuntime JsRuntime { get; set; }

        //protected override void OnAfterRender(bool firstRender)
        //{
        //    if (firstRender)
        //    {
        //        JsRuntime.InvokeVoidAsync("onBlazorReady");
        //    }
        //}
        
        // Reloading the page is better for now
        //private async void RefreshList()
        //{
        //    await SteamSwitcherFuncs.LoadProfiles(JsRuntime);
        //}


        public class Steamuser
        {
            [JsonIgnore] public string SteamId { get; set; }
            [JsonProperty("AccountName", Order = 0)] public string AccName { get; set; }
            [JsonProperty("PersonaName", Order = 1)] public string Name { get; set; }
            [JsonProperty("RememberPassword", Order = 2)] private readonly string _remPass = "1"; // Should always be 1
            [JsonProperty("mostrecent", Order = 3)] public string MostRec = "0";
            [JsonProperty("Timestamp", Order = 4)] public string LastLogin { get; set; }
            [JsonProperty("WantsOfflineMode", Order = 5)] public string OfflineMode = "0";
            [JsonIgnore] public string ImgUrl { get; set; }

            //[JsonIgnore] public string vacStatus { get; set; } // WAS System.Windows.Media.Brush
        }



        //[JSInvokable]
        //public static Task<int> CopyProfileURL()
        //{
        //    return Task.FromResult(new Random().Next());
        //}
    }
}
