using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;

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
            if (Directory.Exists(SteamSwitcherFuncs.SteamFolder(settings)) && File.Exists(SteamSwitcherFuncs.SteamExe(settings)))
            {
                NavManager.NavigateTo("/Steam/");
                return;
            }
            await ShowModal("find:Steam:Steam.exe:SteamSettings");
        }

        public async Task ShowModal(string args)
        {
            await JsRuntime.InvokeAsync<string>("ShowModal", args);
        }
    }
}
