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
        public async Task CheckSteam(IJSRuntime JSRuntime)
        {
            if (Directory.Exists(Steam.FolderPath) && File.Exists(Steam.SteamExe()))
            {
                NavManager.NavigateTo("/Steam/");
                return;
            }
            await GeneralInvocableFuncs.ShowModal(JSRuntime, "find:Steam:Steam.exe:SteamSettings");
        }
    }
}
