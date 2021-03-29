using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index : ComponentBase
    {
        [Inject]
        public IJSRuntime JSRuntime { get; set; }
        private IJSObjectReference _jsModule;
        //private IJSObjectReference _jsModule;
        //protected override async Task OnInitializedAsync()
        //{
        //    _jsModule = await JSRuntime.InvokeAsync<IJSObjectReference>("import", "./js/steam/settings.js");
        //    await _jsModule.InvokeAsync<string>("jsLoadSettings");
        //}
        public async Task CheckSteam()
        {
            //await JsRuntime.InvokeAsync<string>("alert", "TEST");
            JObject settings = GeneralFuncs.LoadSettings("SteamSettings");
            //if (false)
            if (Directory.Exists((string)settings["Path"]) && File.Exists(GeneralFuncs.SteamExe(settings)))
            {
                NavManager.NavigateTo("/Steam/");
            }
            else
            {
                await JsRuntime.InvokeAsync<string>("ShowModal", "find:Steam:Steam.exe:SteamSettings");
                // When found: await JsRuntime.InvokeAsync<string>("Modal_RequestedLocated", "true");
            }
        }
    }
}
