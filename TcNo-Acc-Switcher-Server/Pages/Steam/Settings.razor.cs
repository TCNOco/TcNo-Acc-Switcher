using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class Settings : ComponentBase
    {
        [Inject]
        public IJSRuntime JSRuntime { get; set; }
        private IJSObjectReference _jsModule;
        protected override async Task OnInitializedAsync()
        {
            _jsModule = await JSRuntime.InvokeAsync<IJSObjectReference>("import", "./js/steam/settings.js");
            await _jsModule.InvokeAsync<string>("jsLoadSettings");
        }
        public async Task ClickSaveSettings()
        {
            await _jsModule.InvokeAsync<string>("jsSaveSettings");
            NavManager.NavigateTo("/Steam");
        }
    }
}
